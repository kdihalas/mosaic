// Package compiler implements Mosaic's explicit deterministic compiler pipeline.
package compiler

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kdihalas/mosaic/pkg/capability"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/module"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/policy"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/provenance"
	"github.com/kdihalas/mosaic/pkg/semantic"
	"github.com/kdihalas/mosaic/pkg/semantic/symbols"
	"github.com/kdihalas/mosaic/pkg/syntax/ast"
	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/parser"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/kdihalas/mosaic/pkg/transform"
	"github.com/kdihalas/mosaic/pkg/value"
)

type PhaseName string

const (
	Version         = "0.1.0-dev"
	LanguageVersion = "v1alpha1"
)

const (
	PhaseLex               PhaseName = "lex"
	PhaseParse             PhaseName = "parse"
	PhaseResolveNames      PhaseName = "resolve-names"
	PhaseCheckTypes        PhaseName = "check-types"
	PhaseInstantiate       PhaseName = "instantiate"
	PhaseBuildGraph        PhaseName = "build-graph"
	PhaseApplyVariants     PhaseName = "apply-variants"
	PhaseApplyTransforms   PhaseName = "apply-transforms"
	PhaseDetectConflicts   PhaseName = "detect-conflicts"
	PhaseResolveConflicts  PhaseName = "resolve-conflicts"
	PhaseResolveReferences PhaseName = "resolve-references"
	PhaseValidateGraph     PhaseName = "validate-graph"
	PhaseEvaluatePolicies  PhaseName = "evaluate-policies"
)

type Limits struct {
	MaxDiagnostics       int
	MaxParseDepth        int
	MaxExpressionDepth   int
	MaxModuleDepth       int
	MaxResources         int
	MaxTransformOps      int
	MaxPolicyEvaluations int
}
type NewOptions struct{ Limits Limits }
type Options struct {
	Environment    string
	MaxDiagnostics int
}

// CompilationPackage is a verified package source unit supplied to the compiler.
type CompilationPackage struct {
	Identity     mosaicpackage.Identity
	Version      mosaicpackage.Version
	Aliases      []string
	Root         string
	Files        []source.File
	Manifest     mosaicpackage.Manifest
	Exports      mosaicpackage.ExportSet
	Dependencies []PackageDependency
}

type PackageDependency struct {
	Alias    string
	Identity mosaicpackage.Identity
	Version  mosaicpackage.Version
}

// Input makes source ownership and dependency namespaces explicit.
type Input struct {
	RootProject *project.Project
	Packages    []CompilationPackage
	Environment string
	// Variants are additional baked-in variants applied in declaration order
	// after the variants selected by the environment.
	Variants []string
	Policy   policy.Options
}
type Metadata struct {
	Project         string      `json:"project"`
	CompilerVersion string      `json:"compilerVersion"`
	LanguageVersion string      `json:"languageVersion"`
	SourceDigest    string      `json:"sourceDigest,omitempty"`
	TargetOptions   value.Value `json:"targetOptions"`
}
type Result struct {
	Environment  string
	Graph        *graph.Graph
	Provenance   *provenance.Store
	PolicyReport policy.Report
	Metadata     Metadata
	ParsedFiles  []*ast.File
	Analysis     *semantic.Analysis
	Instances    []module.Instance
	Capabilities []capability.Instance
	Conflicts    []transform.Conflict
	Snapshots    map[PhaseName]*graph.Graph
	BuildInput   Input `json:"-"`
}
type Compiler struct{ limits Limits }

func New(o NewOptions) *Compiler {
	l := o.Limits
	if l.MaxDiagnostics == 0 {
		l.MaxDiagnostics = 100
	}
	if l.MaxParseDepth == 0 {
		l.MaxParseDepth = 256
	}
	if l.MaxExpressionDepth == 0 {
		l.MaxExpressionDepth = 256
	}
	if l.MaxModuleDepth == 0 {
		l.MaxModuleDepth = 64
	}
	if l.MaxResources == 0 {
		l.MaxResources = 100000
	}
	if l.MaxTransformOps == 0 {
		l.MaxTransformOps = 500000
	}
	if l.MaxPolicyEvaluations == 0 {
		l.MaxPolicyEvaluations = 1000000
	}
	return &Compiler{l}
}

type program struct {
	files         []*ast.File
	modules       map[string]*ast.ModuleDeclaration
	capabilities  map[string]*ast.CapabilityDeclaration
	apps          map[string]*ast.ModuleUseDeclaration
	variants      map[string]*ast.VariantDeclaration
	transforms    map[string]*ast.TransformDeclaration
	envs          map[string]*ast.EnvironmentDeclaration
	policies      map[string]*ast.PolicyDeclaration
	tests         map[string]*ast.TestDeclaration
	analysis      *semantic.Analysis
	moduleIDs     map[*ast.ModuleDeclaration]string
	capabilityIDs map[*ast.CapabilityDeclaration]string
	aliases       map[string]mosaicpackage.ExportSet
}

func (c *Compiler) Analyze(ctx context.Context, p *project.Project) (*semantic.Analysis, diagnostics.List) {
	return c.AnalyzeInput(ctx, Input{RootProject: p})
}

func (c *Compiler) AnalyzeInput(ctx context.Context, input Input) (*semantic.Analysis, diagnostics.List) {
	pg, d := c.load(ctx, input)
	if pg == nil {
		return nil, d
	}
	return pg.analysis, d
}
func (c *Compiler) load(ctx context.Context, input Input) (*program, diagnostics.List) {
	if input.RootProject == nil {
		return nil, diagnostics.List{diag("CMP001", "root project is nil", diagnostics.Span{})}
	}
	p := input.RootProject
	pg := &program{modules: map[string]*ast.ModuleDeclaration{}, capabilities: map[string]*ast.CapabilityDeclaration{}, apps: map[string]*ast.ModuleUseDeclaration{}, variants: map[string]*ast.VariantDeclaration{}, transforms: map[string]*ast.TransformDeclaration{}, envs: map[string]*ast.EnvironmentDeclaration{}, policies: map[string]*ast.PolicyDeclaration{}, tests: map[string]*ast.TestDeclaration{}, moduleIDs: map[*ast.ModuleDeclaration]string{}, capabilityIDs: map[*ast.CapabilityDeclaration]string{}, aliases: map[string]mosaicpackage.ExportSet{}}
	table := symbols.New()
	var ds diagnostics.List
	type unit struct {
		files []source.File
		pkg   *CompilationPackage
	}
	units := []unit{{files: p.Files}}
	packages := append([]CompilationPackage(nil), input.Packages...)
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Identity != packages[j].Identity {
			return packages[i].Identity < packages[j].Identity
		}
		return packages[i].Version < packages[j].Version
	})
	for i := range packages {
		for _, alias := range packages[i].Aliases {
			pg.aliases[alias] = packages[i].Exports
		}
	}
	for i := range packages {
		cp := packages[i]
		units = append(units, unit{files: cp.Files, pkg: &cp})
	}
	for _, u := range units {
		for _, original := range u.files {
			f := original
			if u.pkg != nil {
				f.Name = "package/" + u.pkg.Identity.String() + "@" + u.pkg.Version.String() + "/" + original.Name
			}
			select {
			case <-ctx.Done():
				return nil, append(ds, diag("CMP001", ctx.Err().Error(), diagnostics.Span{}))
			default:
			}
			lr := lexer.Lex(f, lexer.Options{MaxDiagnostics: c.limits.MaxDiagnostics})
			ds = append(ds, lr.Diagnostics...)
			pr := parser.Parse(f, lr.Tokens, parser.Options{MaxDiagnostics: c.limits.MaxDiagnostics, MaxParseDepth: c.limits.MaxParseDepth, MaxExpressionDepth: c.limits.MaxExpressionDepth})
			ds = append(ds, pr.Diagnostics...)
			pg.files = append(pg.files, pr.File)
			for _, d := range pr.File.Declarations {
				ds = append(ds, validateWhenPlacement(d)...)
				var k symbols.Kind
				var n string
				switch x := d.(type) {
				case *ast.ModuleDeclaration:
					k, n = symbols.Module, x.Name
					if u.pkg == nil {
						pg.modules[n] = x
						pg.moduleIDs[x] = n
					}
				case *ast.CapabilityDeclaration:
					k, n = symbols.Capability, x.Name
					if u.pkg == nil {
						pg.capabilities[n] = x
						pg.capabilityIDs[x] = n
					}
				case *ast.ModuleUseDeclaration:
					k, n = symbols.Application, x.Alias
					if u.pkg == nil {
						pg.apps[n] = x
					}
				case *ast.VariantDeclaration:
					k, n = symbols.Variant, x.Name
					if u.pkg == nil {
						pg.variants[n] = x
					}
				case *ast.TransformDeclaration:
					k, n = symbols.Transform, x.Name
					if u.pkg == nil {
						pg.transforms[n] = x
					}
				case *ast.EnvironmentDeclaration:
					k, n = symbols.Environment, x.Name
					if u.pkg == nil {
						pg.envs[n] = x
					}
				case *ast.PolicyDeclaration:
					k, n = symbols.Policy, x.Name
					if u.pkg == nil {
						pg.policies[n] = x
					}
				case *ast.TestDeclaration:
					k, n = symbols.Test, x.Name
					if u.pkg == nil {
						pg.tests[n] = x
					}
				case *ast.TypeDeclaration:
					k, n = symbols.Type, x.Name
				case *ast.EnumDeclaration:
					k, n = symbols.Enum, x.Name
				}
				if n != "" {
					id := string(k) + "." + n
					if u.pkg != nil {
						id = mosaicpackage.SymbolID(u.pkg.Identity, u.pkg.Version, string(k), n)
					}
					s := symbols.Symbol{ID: id, Name: n, Kind: k, Source: d.Span()}
					if !table.Add(s) {
						ds = append(ds, diag("SEM001", "duplicate declaration `"+n+"`", d.Span()))
					}
					if u.pkg != nil && u.pkg.Exports.Contains(string(k), n) {
						for _, alias := range u.pkg.Aliases {
							qualified := alias + "." + n
							pg.aliases[alias] = u.pkg.Exports
							switch x := d.(type) {
							case *ast.ModuleDeclaration:
								pg.modules[qualified] = x
								pg.moduleIDs[x] = id
							case *ast.CapabilityDeclaration:
								pg.capabilities[qualified] = x
								pg.capabilityIDs[x] = id
							case *ast.VariantDeclaration:
								pg.variants[qualified] = x
							case *ast.TransformDeclaration:
								pg.transforms[qualified] = x
							case *ast.PolicyDeclaration:
								pg.policies[qualified] = x
							case *ast.TestDeclaration:
								pg.tests[qualified] = x
							}
						}
					}
				}
			}
		}
	}
	for _, a := range pg.apps {
		if _, ok := pg.modules[a.Module]; !ok {
			code := "SEM003"
			message := "unknown module `" + a.Module + "`"
			if parts := strings.SplitN(a.Module, ".", 2); len(parts) == 2 {
				if _, known := pg.aliases[parts[0]]; known {
					code = "PKG031"
					message = "symbol `" + parts[1] + "` is private to package alias `" + parts[0] + "`"
				}
			}
			ds = append(ds, diag(code, message, a.Span()))
		}
	}
	pg.analysis = &semantic.Analysis{Files: pg.files, Symbols: table.List()}
	return pg, ds.Sorted()
}

func validateWhenPlacement(declaration ast.Declaration) diagnostics.List {
	var ds diagnostics.List
	var walkConstruction func([]ast.Statement)
	walkConstruction = func(statements []ast.Statement) {
		for _, statement := range statements {
			switch typed := statement.(type) {
			case *ast.WhenStatement:
				walkConstruction(typed.Body)
			case *ast.ResourceDeclaration:
				walkConstruction(typed.Body)
			case *ast.BlockDeclaration:
				walkConstruction(typed.Body)
			default:
				for _, conditional := range nestedWhens(statement) {
					ds = append(ds, diag("SEM047", "when is not supported in this nested body", conditional.Span()))
				}
			}
		}
	}
	var walkVariant func([]ast.Statement)
	walkVariant = func(statements []ast.Statement) {
		for _, statement := range statements {
			if conditional, ok := statement.(*ast.WhenStatement); ok {
				walkVariant(conditional.Body)
				continue
			}
			for _, conditional := range nestedWhens(statement) {
				ds = append(ds, diag("SEM047", "when must wrap a variant operation", conditional.Span()))
			}
		}
	}
	switch typed := declaration.(type) {
	case *ast.ModuleDeclaration:
		walkConstruction(typed.Body)
	case *ast.CapabilityDeclaration:
		walkConstruction(typed.Body)
	case *ast.VariantDeclaration:
		walkVariant(typed.Body)
	default:
		for _, conditional := range declarationWhens(declaration) {
			ds = append(ds, diag("SEM047", "when is only supported in module construction and variants", conditional.Span()))
		}
	}
	return ds
}

func declarationWhens(declaration ast.Declaration) []*ast.WhenStatement {
	var body []ast.Statement
	switch typed := declaration.(type) {
	case *ast.TypeDeclaration:
		body = typed.Body
	case *ast.ModuleUseDeclaration:
		body = typed.Body
	case *ast.EnvironmentDeclaration:
		body = typed.Body
	case *ast.TransformDeclaration:
		body = typed.Body
	case *ast.PolicyDeclaration:
		body = typed.Body
	case *ast.TestDeclaration:
		body = typed.Body
	}
	var out []*ast.WhenStatement
	for _, statement := range body {
		if conditional, ok := statement.(*ast.WhenStatement); ok {
			out = append(out, conditional)
		}
		out = append(out, nestedWhens(statement)...)
	}
	return out
}

func nestedWhens(statement ast.Statement) []*ast.WhenStatement {
	var body []ast.Statement
	switch typed := statement.(type) {
	case *ast.WhenStatement:
		body = typed.Body
	case *ast.ResourceDeclaration:
		body = typed.Body
	case *ast.BlockDeclaration:
		body = typed.Body
	case *ast.OperationStatement:
		body = typed.Body
	case *ast.EnableStatement:
		body = typed.Body
	case *ast.AddStatement:
		body = typed.Body
	case *ast.MergeStatement:
		body = typed.Body
	case *ast.RequireStatement:
		body = typed.Body
	case *ast.DenyStatement:
		body = typed.Body
	case *ast.WarnStatement:
		body = typed.Body
	case *ast.SelectStatement:
		body = typed.Body
	}
	var out []*ast.WhenStatement
	for _, child := range body {
		if conditional, ok := child.(*ast.WhenStatement); ok {
			out = append(out, conditional)
		}
		out = append(out, nestedWhens(child)...)
	}
	return out
}
func (c *Compiler) Compile(ctx context.Context, p *project.Project, o Options) (*Result, diagnostics.List) {
	return c.CompileInput(ctx, Input{RootProject: p, Environment: o.Environment})
}

func (c *Compiler) CompileInput(ctx context.Context, input Input) (*Result, diagnostics.List) {
	p := input.RootProject
	pg, ds := c.load(ctx, input)
	if pg == nil || ds.HasErrors() {
		return nil, ds
	}
	env, ok := pg.envs[input.Environment]
	if !ok {
		return nil, append(ds, diag("SEM004", "unknown environment `"+input.Environment+"`", diagnostics.Span{SourceName: input.Environment}))
	}
	g := graph.New()
	prov := provenance.New()
	r := &Result{Environment: input.Environment, Graph: g, Provenance: prov, Metadata: Metadata{Project: p.Name, CompilerVersion: Version, LanguageVersion: LanguageVersion, SourceDigest: sourceDigest(input), TargetOptions: value.Object(nil)}, ParsedFiles: pg.files, Analysis: pg.analysis, Snapshots: map[PhaseName]*graph.Graph{}, BuildInput: cloneInput(input)}
	for _, s := range env.Body {
		if b, ok := s.(*ast.BlockDeclaration); ok && b.Name == "target" {
			if v, e := statementsObject(b.Body, semantic.Context{}); e == nil {
				r.Metadata.TargetOptions = v
			}
		}
	}
	aliases := envUses(env)
	sort.Strings(aliases)
	for _, alias := range aliases {
		app, yes := pg.apps[alias]
		if !yes {
			ds = append(ds, diag("SEM005", "unknown application `"+alias+"`", env.Span()))
			continue
		}
		m := pg.modules[app.Module]
		inst, e := c.instantiate(g, prov, m, app, pg.moduleIDs[m])
		if e != nil {
			ds = append(ds, diag(errorCode(e, "CMP010"), e.Error(), app.Span()))
		} else {
			r.Instances = append(r.Instances, inst)
		}
	}
	r.Snapshots[PhaseBuildGraph] = g.Snapshot()
	if len(g.List()) > c.limits.MaxResources {
		ds = append(ds, diag("CMP011", "resource limit exceeded", env.Span()))
	}
	writes := map[string][]transform.FieldWrite{}
	exportIndex := instantiatedExports(r.Instances)
	baseGraph := r.Snapshots[PhaseBuildGraph]
	applied := envApplies(env)
	sort.Strings(applied)
	seenVariants := make(map[string]bool, len(applied)+len(input.Variants))
	for _, name := range applied {
		seenVariants[name] = true
	}
	for _, name := range input.Variants {
		if seenVariants[name] {
			ds = append(ds, diag("SEM007", "duplicate selected variant or transform `"+name+"`", env.Span()))
			continue
		}
		seenVariants[name] = true
		applied = append(applied, name)
	}
	type selectedOperation struct {
		statement ast.Statement
		owner     provenance.Owner
		narrowed  map[graph.ResourceID]bool
	}
	var selectedOperations []selectedOperation
	capabilityKeys := map[string]bool{}
	for _, name := range applied {
		var body []ast.Statement
		var ownerKind string
		if v := pg.variants[name]; v != nil {
			body = v.Body
			ownerKind = "variant"
		} else if t := pg.transforms[name]; t != nil {
			body = t.Body
			ownerKind = "transform"
		} else {
			ds = append(ds, diag("SEM006", "unknown variant or transform `"+name+"`", env.Span()))
			continue
		}
		guarded := make([]guardedOperation, 0, len(body))
		if ownerKind == "variant" {
			var err error
			guarded, err = guardedVariantStatements(body, variantContext(baseGraph, exportIndex), nil)
			if err != nil {
				ds = append(ds, diag(errorCode(err, "SEM043"), err.Error(), pg.variants[name].Span()))
				continue
			}
		} else {
			for _, statement := range body {
				if _, conditional := statement.(*ast.WhenStatement); conditional {
					ds = append(ds, diag("SEM047", "when is not supported in transforms", statement.Span()))
					continue
				}
				guarded = append(guarded, guardedOperation{statement: statement})
			}
		}
		for _, operation := range guarded {
			selectedOperations = append(selectedOperations, selectedOperation{statement: operation.statement, owner: provenance.Owner{Kind: ownerKind, Name: name}, narrowed: operation.narrowed})
		}
	}
	ops := 0
	for _, operation := range selectedOperations {
		s := operation.statement
		ops++
		if ops > c.limits.MaxTransformOps {
			ds = append(ds, diag("CMP012", "transform operation limit exceeded", s.Span()))
			break
		}
		if err := validateExternalMutationTarget(baseGraph, exportIndex, operation.narrowed, s); err != nil {
			ds = append(ds, diag(errorCode(err, "SEM046"), err.Error(), s.Span()))
			continue
		}
		if enabled, ok := s.(*ast.EnableStatement); ok {
			op := (*ast.OperationStatement)(enabled)
			if declaration := pg.capabilities[op.Name]; declaration != nil {
				instance, err := c.instantiateCapability(g, prov, declaration, op, pg.capabilityIDs[declaration], capabilityKeys, operation.owner)
				if err != nil {
					ds = append(ds, diag(errorCode(err, "TRN001"), err.Error(), s.Span()))
					continue
				}
				r.Capabilities = append(r.Capabilities, instance)
				continue
			}
			if strings.Contains(op.Name, ".") {
				ds = append(ds, diag("SEM006", "unknown package capability `"+op.Name+"`", s.Span()))
				continue
			}
		}
		ws, err := c.applyStatement(g, prov, s, operation.owner, false)
		if err != nil {
			ds = append(ds, diag("TRN001", err.Error(), s.Span()))
		}
		for _, w := range ws {
			k := string(w.ResourceID) + "\x00" + w.Path.String()
			writes[k] = append(writes[k], w)
		}
	}
	if len(g.List()) > c.limits.MaxResources {
		ds = append(ds, diag("CMP011", "resource limit exceeded after capability expansion", env.Span()))
	}
	r.Snapshots[PhaseApplyVariants] = g.Snapshot()
	r.Snapshots[PhaseApplyTransforms] = g.Snapshot()
	for _, group := range writes {
		if conflict(group) {
			sort.Slice(group, func(i, j int) bool { return group[i].Owner.Name < group[j].Owner.Name })
			r.Conflicts = append(r.Conflicts, transform.Conflict{ResourceID: group[0].ResourceID, Path: group[0].Path, Writes: group})
		}
	}
	for _, s := range env.Body {
		if _, ok := s.(*ast.ResolveStatement); ok {
			if e := validateExternalMutationTarget(baseGraph, exportIndex, nil, s); e != nil {
				ds = append(ds, diag(errorCode(e, "SEM046"), e.Error(), s.Span()))
				continue
			}
			ws, e := c.applyStatement(g, prov, s, provenance.Owner{Kind: "environment", Name: env.Name}, true)
			if e != nil {
				ds = append(ds, diag("TRN002", e.Error(), s.Span()))
				continue
			}
			for _, w := range ws {
				for i := range r.Conflicts {
					if r.Conflicts[i].ResourceID == w.ResourceID && r.Conflicts[i].Path.String() == w.Path.String() {
						r.Conflicts[i].Resolved = true
					}
				}
			}
		}
	}
	for _, x := range r.Conflicts {
		if !x.Resolved {
			ds = append(ds, diag("SEM042", "conflicting assignments to "+string(x.ResourceID)+"."+x.Path.String(), x.Writes[0].Source))
		}
	}
	r.Snapshots[PhaseResolveConflicts] = g.Snapshot()
	c.resolveReferences(g, &ds)
	r.Snapshots[PhaseResolveReferences] = g.Snapshot()
	if e := g.Validate(); e != nil {
		ds = append(ds, diag("GRF001", e.Error(), env.Span()))
	}
	r.PolicyReport = c.policies(pg, env, input.Policy, g, prov, &ds)
	r.Snapshots[PhaseEvaluatePolicies] = g.Snapshot()
	r.Graph = g.Snapshot()
	return r, ds.Sorted()
}

func cloneInput(input Input) Input {
	out := input
	out.Variants = append([]string(nil), input.Variants...)
	out.Policy.Include = append([]string(nil), input.Policy.Include...)
	out.Policy.Exclude = append([]string(nil), input.Policy.Exclude...)
	out.Packages = append([]CompilationPackage(nil), input.Packages...)
	for i := range out.Packages {
		out.Packages[i].Aliases = append([]string(nil), input.Packages[i].Aliases...)
		out.Packages[i].Files = append([]source.File(nil), input.Packages[i].Files...)
		for j := range out.Packages[i].Files {
			out.Packages[i].Files[j].Content = append([]byte(nil), input.Packages[i].Files[j].Content...)
		}
		out.Packages[i].Dependencies = append([]PackageDependency(nil), input.Packages[i].Dependencies...)
	}
	if input.RootProject != nil {
		root := *input.RootProject
		root.Root = ""
		root.Files = append([]source.File(nil), input.RootProject.Files...)
		for i := range root.Files {
			root.Files[i].Content = append([]byte(nil), input.RootProject.Files[i].Content...)
		}
		out.RootProject = &root
	}
	return out
}
func (c *Compiler) instantiate(g *graph.Graph, ps *provenance.Store, m *ast.ModuleDeclaration, a *ast.ModuleUseDeclaration, moduleID string) (module.Instance, error) {
	inputs, err := statementsObject(a.Body, semantic.Context{})
	if err != nil {
		return module.Instance{}, err
	}
	ctx := semantic.Context{Values: map[string]value.Value{"input": inputs}}
	ctx.ResolvePath = func(p []string) (value.Value, bool) {
		if len(p) > 0 && p[0] == "input" {
			v := inputs
			for _, k := range p[1:] {
				var ok bool
				v, ok = v.Get(k)
				if !ok {
					return value.Value{}, false
				}
			}
			return v, true
		}
		if len(p) == 2 {
			return value.Reference(value.ReferenceValue{Target: "application." + a.Alias + "." + p[0] + "." + p[1], Type: p[0]}), true
		}
		return value.Value{}, false
	}
	if moduleID == "" {
		moduleID = m.Name
	}
	contract, err := analyzeModuleContract(m.Body)
	if err != nil {
		return module.Instance{}, err
	}
	if err := validateConstructionGuards(m.Body, ctx); err != nil {
		return module.Instance{}, err
	}
	selected, err := guardedStatements(m.Body, ctx)
	if err != nil {
		return module.Instance{}, err
	}
	extensions, protected := mutationPoints(selected)
	inst := module.Instance{Module: moduleID, Alias: a.Alias}
	presentResources := map[string]bool{}
	selectedExports := map[string]bool{}
	for _, statement := range selected {
		if exported, ok := statement.(*ast.ExportStatement); ok {
			if path, yes := semantic.Path(exported.Target); yes && len(path) >= 2 {
				selectedExports[path[0]+"."+path[1]] = true
			}
		}
	}
	for _, s := range selected {
		rd, ok := s.(*ast.ResourceDeclaration)
		if !ok {
			continue
		}
		fields, e := statementsObject(rd.Body, ctx)
		if e != nil {
			return inst, e
		}
		id := graph.ResourceID("application." + a.Alias + "." + rd.Kind + "." + rd.Name)
		name := rd.Name
		if n, ok := fields.Get("name"); ok {
			if x, yes := n.StringValue(); yes {
				name = x
			}
		}
		key := rd.Kind + "." + rd.Name
		presentResources[key] = true
		exported := contract.implicit || selectedExports[key]
		r := graph.Resource{ID: id, Type: coreType(rd.Kind), Name: name, Fields: fields, Metadata: graph.Metadata{Module: moduleID, Source: rd.Span(), Exported: exported, Extensions: extensions[key], Protected: protected[key]}}
		if v, ok := fields.Get("labels"); ok {
			r.Labels = stringMap(v)
		}
		if v, ok := fields.Get("annotations"); ok {
			r.Annotations = stringMap(v)
		}
		if e := g.Add(r); e != nil {
			return inst, e
		}
		inst.Resources = append(inst.Resources, id)
		ps.Add(provenance.Event{ResourceID: id, Action: provenance.ResourceCreated, Current: fields, Owner: provenance.Owner{Kind: "module", Name: moduleID}, Source: rd.Span()})
		for _, st := range rd.Body {
			as, ok := st.(*ast.AssignmentStatement)
			if !ok {
				continue
			}
			path, ok := semantic.Path(as.Target)
			if !ok || len(path) != 1 {
				continue
			}
			current, ok := fields.Get(path[0])
			if !ok {
				continue
			}
			if inputPath, yes := semantic.Path(as.Value); yes && len(inputPath) > 1 && inputPath[0] == "input" {
				ps.Add(provenance.Event{ResourceID: id, FieldPath: graph.FieldPath{path[0]}, Action: provenance.InputSupplied, Current: current, Owner: provenance.Owner{Kind: "application", Name: a.Alias}, Source: inputSource(a, inputPath[1])})
			}
			ps.Add(provenance.Event{ResourceID: id, FieldPath: graph.FieldPath{path[0]}, Action: provenance.ModuleAssigned, Current: current, Owner: provenance.Owner{Kind: "module", Name: m.Name}, Source: as.Span()})
		}
	}
	for _, definition := range contract.exports {
		present := presentResources[definition.key] && (contract.implicit || selectedExports[definition.key])
		if !definition.optional && !present {
			return inst, coded("SEM047", "required export %s is not present", definition.key)
		}
		inst.Exports = append(inst.Exports, module.Export{
			ResourceID: graph.ResourceID("application." + a.Alias + "." + definition.key),
			Optional:   definition.optional,
			Present:    present,
		})
	}
	sort.Slice(inst.Exports, func(i, j int) bool { return inst.Exports[i].ResourceID < inst.Exports[j].ResourceID })
	return inst, nil
}

func (c *Compiler) instantiateCapability(g *graph.Graph, ps *provenance.Store, declaration *ast.CapabilityDeclaration, operation *ast.OperationStatement, capabilityID string, instanceKeys map[string]bool, owner provenance.Owner) (capability.Instance, error) {
	if operation.Identity == "" {
		return capability.Instance{}, coded("SEM047", "package capability %s requires `as <instance>`", operation.Name)
	}
	if declaration.Parameter != nil && declaration.Parameter.Name != "input" {
		return capability.Instance{}, coded("SEM047", "capability parameter must be named input")
	}
	inputs, err := statementsObject(operation.Body, semantic.Context{ResolvePath: func(path []string) (value.Value, bool) {
		id, field, pathErr := targetPath(path)
		if pathErr != nil {
			return value.Value{}, false
		}
		return value.Reference(value.ReferenceValue{Target: string(id), Field: field}), true
	}})
	if err != nil {
		return capability.Instance{}, err
	}
	targetValue, ok := inputs.Get("target")
	if !ok {
		return capability.Instance{}, coded("SEM047", "package capability %s requires target input", operation.Name)
	}
	targetReference, ok := targetValue.ReferenceValue()
	if !ok || len(targetReference.Field) != 0 {
		return capability.Instance{}, coded("SEM047", "package capability target must reference a resource")
	}
	parts := strings.Split(targetReference.Target, ".")
	if len(parts) < 3 || parts[0] != "application" {
		return capability.Instance{}, coded("SEM047", "invalid package capability target")
	}
	application := parts[1]
	instanceKey := application + "\x00" + operation.Identity
	if instanceKeys[instanceKey] {
		return capability.Instance{}, coded("SEM047", "duplicate capability instance %s for application %s", operation.Identity, application)
	}
	if _, found := g.Get(graph.ResourceID(targetReference.Target)); !found {
		return capability.Instance{}, coded("SEM047", "package capability target %s does not exist", targetReference.Target)
	}
	if capabilityID == "" {
		capabilityID = declaration.Name
	}
	contract, err := analyzeModuleContract(declaration.Body)
	if err != nil {
		return capability.Instance{}, err
	}
	ctx := semantic.Context{Values: map[string]value.Value{"input": inputs}}
	ctx.ResolvePath = func(path []string) (value.Value, bool) {
		if len(path) > 0 && path[0] == "input" {
			current := inputs
			for _, key := range path[1:] {
				var found bool
				current, found = current.Get(key)
				if !found {
					return value.Value{}, false
				}
			}
			return current, true
		}
		if len(path) == 2 {
			id := fmt.Sprintf("application.%s.capability.%s.%s.%s", application, operation.Identity, path[0], path[1])
			return value.Reference(value.ReferenceValue{Target: id, Type: path[0]}), true
		}
		return value.Value{}, false
	}
	if err := validateConstructionGuards(declaration.Body, ctx); err != nil {
		return capability.Instance{}, err
	}
	selected, err := guardedStatements(declaration.Body, ctx)
	if err != nil {
		return capability.Instance{}, err
	}
	extensions, protected := mutationPoints(selected)
	selectedExports := map[string]bool{}
	for _, statement := range selected {
		if exported, yes := statement.(*ast.ExportStatement); yes {
			if path, valid := semantic.Path(exported.Target); valid && len(path) >= 2 {
				selectedExports[path[0]+"."+path[1]] = true
			}
		}
	}
	instance := capability.Instance{Capability: capabilityID, Application: application, Name: operation.Identity}
	presentResources := map[string]bool{}
	var staged []graph.Resource
	for _, statement := range selected {
		resourceDeclaration, yes := statement.(*ast.ResourceDeclaration)
		if !yes {
			continue
		}
		fields, fieldErr := statementsObject(resourceDeclaration.Body, ctx)
		if fieldErr != nil {
			return instance, fieldErr
		}
		key := resourceDeclaration.Kind + "." + resourceDeclaration.Name
		id := graph.ResourceID(fmt.Sprintf("application.%s.capability.%s.%s", application, operation.Identity, key))
		name := resourceDeclaration.Name
		if configured, found := fields.Get("name"); found {
			if rendered, valid := configured.StringValue(); valid {
				name = rendered
			}
		}
		presentResources[key] = true
		resource := graph.Resource{
			ID: id, Type: coreType(resourceDeclaration.Kind), Name: name, Fields: fields,
			Metadata: graph.Metadata{Module: capabilityID, Source: resourceDeclaration.Span(), Exported: contract.implicit || selectedExports[key], Extensions: extensions[key], Protected: protected[key]},
		}
		if labels, found := fields.Get("labels"); found {
			resource.Labels = stringMap(labels)
		}
		if annotations, found := fields.Get("annotations"); found {
			resource.Annotations = stringMap(annotations)
		}
		staged = append(staged, resource)
		instance.Resources = append(instance.Resources, id)
	}
	seenResources := map[graph.ResourceID]bool{}
	for _, resource := range staged {
		if seenResources[resource.ID] {
			return instance, fmt.Errorf("duplicate capability resource %s", resource.ID)
		}
		seenResources[resource.ID] = true
		if _, exists := g.Get(resource.ID); exists {
			return instance, fmt.Errorf("duplicate resource %s", resource.ID)
		}
	}
	for _, resource := range staged {
		if err := g.Add(resource); err != nil {
			return instance, err
		}
		ps.Add(provenance.Event{ResourceID: resource.ID, Action: provenance.CapabilityEnabled, Current: resource.Fields, Owner: owner, Source: operation.Span()})
		ps.Add(provenance.Event{ResourceID: resource.ID, Action: provenance.ResourceCreated, Current: resource.Fields, Owner: provenance.Owner{Kind: "capability", Name: capabilityID + " as " + operation.Identity}, Source: resource.Metadata.Source})
	}
	instanceKeys[instanceKey] = true
	for _, definition := range contract.exports {
		present := presentResources[definition.key] && (contract.implicit || selectedExports[definition.key])
		if !definition.optional && !present {
			return instance, coded("SEM047", "required capability export %s is not present", definition.key)
		}
		instance.Exports = append(instance.Exports, module.Export{
			ResourceID: graph.ResourceID(fmt.Sprintf("application.%s.capability.%s.%s", application, operation.Identity, definition.key)),
			Optional:   definition.optional, Present: present,
		})
	}
	sort.Slice(instance.Resources, func(i, j int) bool { return instance.Resources[i] < instance.Resources[j] })
	sort.Slice(instance.Exports, func(i, j int) bool { return instance.Exports[i].ResourceID < instance.Exports[j].ResourceID })
	return instance, nil
}

type contractExport struct {
	key      string
	optional bool
}

type moduleContract struct {
	implicit bool
	exports  []contractExport
}

type guardedDeclaration struct {
	key    string
	guards string
}

func analyzeModuleContract(statements []ast.Statement) (moduleContract, error) {
	resources := map[string]guardedDeclaration{}
	exports := map[string]contractExport{}
	var mutations []guardedDeclaration
	explicit := false
	var walk func([]ast.Statement, []string)
	walk = func(items []ast.Statement, guards []string) {
		for _, statement := range items {
			switch typed := statement.(type) {
			case *ast.WhenStatement:
				id := fmt.Sprintf("%s:%d", typed.Span().SourceName, typed.Span().Start.Offset)
				walk(typed.Body, append(append([]string(nil), guards...), id))
			case *ast.ResourceDeclaration:
				key := typed.Kind + "." + typed.Name
				resources[key] = guardedDeclaration{key: key, guards: strings.Join(guards, "\x00")}
			case *ast.ExportStatement:
				explicit = true
				if path, ok := semantic.Path(typed.Target); ok && len(path) >= 2 {
					key := path[0] + "." + path[1]
					candidate := contractExport{key: key, optional: len(guards) > 0}
					if existing, found := exports[key]; found {
						candidate.optional = existing.optional && candidate.optional
					}
					exports[key] = candidate
				}
			case *ast.ExtensionStatement:
				if path, ok := semantic.Path(typed.Target); ok && len(path) >= 2 {
					mutations = append(mutations, guardedDeclaration{key: path[0] + "." + path[1], guards: strings.Join(guards, "\x00")})
				}
			case *ast.ProtectedStatement:
				if path, ok := semantic.Path(typed.Target); ok && len(path) >= 2 {
					mutations = append(mutations, guardedDeclaration{key: path[0] + "." + path[1], guards: strings.Join(guards, "\x00")})
				}
			}
		}
	}
	walk(statements, nil)
	for _, mutation := range mutations {
		resource, ok := resources[mutation.key]
		if !ok {
			return moduleContract{}, coded("SEM047", "conditional contract target %s does not exist", mutation.key)
		}
		if mutation.guards != resource.guards {
			return moduleContract{}, coded("SEM047", "extension or protected declaration for %s must use the same when guards as its resource", mutation.key)
		}
	}
	contract := moduleContract{implicit: !explicit}
	if !explicit {
		for key, resource := range resources {
			contract.exports = append(contract.exports, contractExport{key: key, optional: resource.guards != ""})
		}
	} else {
		for key, exported := range exports {
			resource, ok := resources[key]
			if !ok {
				return moduleContract{}, coded("SEM047", "export target %s does not exist", key)
			}
			exported.optional = exported.optional || resource.guards != ""
			contract.exports = append(contract.exports, exported)
		}
	}
	sort.Slice(contract.exports, func(i, j int) bool { return contract.exports[i].key < contract.exports[j].key })
	return contract, nil
}

func guardedStatements(statements []ast.Statement, ctx semantic.Context) ([]ast.Statement, error) {
	var out []ast.Statement
	for _, statement := range statements {
		switch typed := statement.(type) {
		case *ast.WhenStatement:
			condition, err := semantic.Evaluate(typed.Condition, ctx)
			if err != nil {
				return nil, conditionError(err)
			}
			enabled, ok := condition.BoolValue()
			if !ok {
				return nil, coded("SEM043", "when condition must be bool")
			}
			if enabled {
				nested, err := guardedStatements(typed.Body, ctx)
				if err != nil {
					return nil, err
				}
				out = append(out, nested...)
			}
		case *ast.ResourceDeclaration:
			body, err := guardedStatements(typed.Body, ctx)
			if err != nil {
				return nil, err
			}
			clone := *typed
			clone.Body = body
			out = append(out, &clone)
		case *ast.BlockDeclaration:
			body, err := guardedStatements(typed.Body, ctx)
			if err != nil {
				return nil, err
			}
			clone := *typed
			clone.Body = body
			out = append(out, &clone)
		default:
			out = append(out, statement)
		}
	}
	return out, nil
}

func validateConstructionGuards(statements []ast.Statement, ctx semantic.Context) error {
	for _, statement := range statements {
		var body []ast.Statement
		switch typed := statement.(type) {
		case *ast.WhenStatement:
			if err := validateConstructionExpression(typed.Condition); err != nil {
				return err
			}
			condition, err := semantic.Evaluate(typed.Condition, ctx)
			if err != nil {
				return conditionError(err)
			}
			if _, ok := condition.BoolValue(); !ok {
				return coded("SEM043", "when condition must be bool")
			}
			body = typed.Body
		case *ast.ResourceDeclaration:
			body = typed.Body
		case *ast.BlockDeclaration:
			body = typed.Body
		}
		if len(body) > 0 {
			if err := validateConstructionGuards(body, ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateConstructionExpression(expression ast.Expression) error {
	if call, ok := expression.(*ast.CallExpression); ok {
		for _, argument := range call.Arguments {
			if err := validateConstructionExpression(argument); err != nil {
				return err
			}
		}
		return nil
	}
	if path, ok := semantic.Path(expression); ok {
		if len(path) == 0 || path[0] != "input" {
			return coded("SEM045", "construction when conditions may only read input values")
		}
		return nil
	}
	switch typed := expression.(type) {
	case *ast.BinaryExpression:
		if err := validateConstructionExpression(typed.Left); err != nil {
			return err
		}
		return validateConstructionExpression(typed.Right)
	case *ast.UnaryExpression:
		return validateConstructionExpression(typed.Operand)
	case *ast.ParenthesisedExpression:
		return validateConstructionExpression(typed.Expression)
	case *ast.IndexExpression:
		if err := validateConstructionExpression(typed.Object); err != nil {
			return err
		}
		return validateConstructionExpression(typed.Index)
	case *ast.ListExpression:
		for _, element := range typed.Elements {
			if err := validateConstructionExpression(element); err != nil {
				return err
			}
		}
	case *ast.ObjectExpression:
		for _, entry := range typed.Entries {
			if err := validateConstructionExpression(entry.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func mutationPoints(statements []ast.Statement) (map[string][]graph.FieldPath, map[string][]graph.FieldPath) {
	extensions := map[string][]graph.FieldPath{}
	protected := map[string][]graph.FieldPath{}
	for _, statement := range statements {
		var targetExpression ast.Expression
		var destination map[string][]graph.FieldPath
		switch typed := statement.(type) {
		case *ast.ExtensionStatement:
			targetExpression, destination = typed.Target, extensions
		case *ast.ProtectedStatement:
			targetExpression, destination = typed.Target, protected
		default:
			continue
		}
		path, ok := semantic.Path(targetExpression)
		if !ok || len(path) < 3 {
			continue
		}
		key := path[0] + "." + path[1]
		destination[key] = append(destination[key], graph.FieldPath(path[2:]))
	}
	for _, collection := range []map[string][]graph.FieldPath{extensions, protected} {
		for key := range collection {
			sort.Slice(collection[key], func(i, j int) bool { return collection[key][i].String() < collection[key][j].String() })
		}
	}
	return extensions, protected
}
func inputSource(a *ast.ModuleUseDeclaration, name string) diagnostics.Span {
	for _, s := range a.Body {
		if x, ok := s.(*ast.AssignmentStatement); ok {
			if p, yes := semantic.Path(x.Target); yes && len(p) == 1 && p[0] == name {
				return x.Span()
			}
		}
	}
	return a.Span()
}
func coreType(k string) graph.TypeName {
	switch k {
	case "config":
		return "core.Config"
	case "serviceAccount":
		return "core.ServiceAccount"
	case "workload":
		return "core.Workload"
	case "expose", "exposure":
		return "core.Exposure"
	case "resource":
		return "kubernetes.CustomResource"
	default:
		return graph.TypeName("core." + strings.ToUpper(k[:1]) + k[1:])
	}
}
func statementsObject(ss []ast.Statement, c semantic.Context) (value.Value, error) {
	m := map[string]value.Value{}
	for _, s := range ss {
		switch x := s.(type) {
		case *ast.AssignmentStatement:
			p, ok := semantic.Path(x.Target)
			if k, q := x.Target.(*ast.StringLiteral); q {
				p = []string{k.Value}
				ok = true
			}
			if !ok || len(p) != 1 {
				return value.Value{}, fmt.Errorf("object assignment must name one field")
			}
			if _, dup := m[p[0]]; dup {
				return value.Value{}, fmt.Errorf("duplicate object field %s", p[0])
			}
			v, e := semantic.Evaluate(x.Value, c)
			if e != nil {
				return value.Value{}, e
			}
			m[p[0]] = v
		case *ast.BlockDeclaration:
			if _, dup := m[x.Name]; dup {
				return value.Value{}, fmt.Errorf("duplicate object field %s", x.Name)
			}
			v, e := statementsObject(x.Body, c)
			if e != nil {
				return value.Value{}, e
			}
			m[x.Name] = v
		}
	}
	return value.Object(m), nil
}
func stringMap(v value.Value) map[string]string {
	m, ok := v.ObjectValue()
	if !ok {
		return nil
	}
	r := map[string]string{}
	for k, x := range m {
		if s, yes := x.StringValue(); yes {
			r[k] = s
		}
	}
	return r
}
func envUses(e *ast.EnvironmentDeclaration) []string {
	var x []string
	for _, s := range e.Body {
		if u, ok := s.(*ast.UseStatement); ok {
			x = append(x, u.Name)
		}
	}
	return x
}
func envApplies(e *ast.EnvironmentDeclaration) []string {
	var x []string
	for _, s := range e.Body {
		if a, ok := s.(*ast.ApplyStatement); ok {
			x = append(x, a.Name)
		}
	}
	return x
}
func op(s ast.Statement) (*ast.OperationStatement, bool) {
	switch x := s.(type) {
	case *ast.SetStatement:
		v := ast.OperationStatement(*x)
		return &v, true
	case *ast.ReplaceStatement:
		v := ast.OperationStatement(*x)
		return &v, true
	case *ast.DeleteStatement:
		v := ast.OperationStatement(*x)
		return &v, true
	case *ast.AppendStatement:
		v := ast.OperationStatement(*x)
		return &v, true
	case *ast.MergeStatement:
		v := ast.OperationStatement(*x)
		return &v, true
	case *ast.AddStatement:
		v := ast.OperationStatement(*x)
		return &v, true
	case *ast.EnableStatement:
		v := ast.OperationStatement(*x)
		return &v, true
	case *ast.ResolveStatement:
		v := ast.OperationStatement(*x)
		return &v, true
	}
	return nil, false
}

type guardedOperation struct {
	statement ast.Statement
	narrowed  map[graph.ResourceID]bool
}

func instantiatedExports(instances []module.Instance) map[graph.ResourceID]module.Export {
	out := map[graph.ResourceID]module.Export{}
	for _, instance := range instances {
		for _, exported := range instance.Exports {
			out[exported.ResourceID] = exported
		}
	}
	return out
}

func variantContext(g *graph.Graph, exports map[graph.ResourceID]module.Export) semantic.Context {
	return semantic.Context{
		StrictPaths: true,
		ResolvePath: func(path []string) (value.Value, bool) {
			if len(path) < 3 {
				return value.Value{}, false
			}
			id := graph.ResourceID("application." + path[0] + "." + path[1] + "." + path[2])
			if exported, known := exports[id]; known {
				if !exported.Present {
					return value.Value{}, false
				}
			} else if resource, found := g.Get(id); !found || (resource.Metadata.Module != "" && !resource.Metadata.Exported) {
				return value.Value{}, false
			}
			if len(path) == 3 {
				resource, found := g.Get(id)
				if !found {
					return value.Value{}, false
				}
				return resource.Fields, true
			}
			return g.ReadField(id, graph.FieldPath(path[3:]))
		},
		PresentPath: func(path []string) (bool, bool) {
			if len(path) != 3 {
				return false, false
			}
			id := graph.ResourceID("application." + path[0] + "." + path[1] + "." + path[2])
			exported, ok := exports[id]
			if !ok || !exported.Optional {
				return false, false
			}
			return exported.Present, true
		},
	}
}

func guardedVariantStatements(statements []ast.Statement, ctx semantic.Context, narrowed map[graph.ResourceID]bool) ([]guardedOperation, error) {
	var out []guardedOperation
	for _, statement := range statements {
		conditional, ok := statement.(*ast.WhenStatement)
		if !ok {
			out = append(out, guardedOperation{statement: statement, narrowed: cloneNarrowed(narrowed)})
			continue
		}
		if err := validateOptionalExpression(conditional.Condition, ctx, narrowed); err != nil {
			return nil, err
		}
		condition, err := semantic.Evaluate(conditional.Condition, ctx)
		if err != nil {
			return nil, conditionError(err)
		}
		enabled, yes := condition.BoolValue()
		if !yes {
			return nil, coded("SEM043", "when condition must be bool")
		}
		if !enabled {
			continue
		}
		next := cloneNarrowed(narrowed)
		for _, id := range definitelyPresent(conditional.Condition) {
			next[id] = true
		}
		nested, err := guardedVariantStatements(conditional.Body, ctx, next)
		if err != nil {
			return nil, err
		}
		out = append(out, nested...)
	}
	return out, nil
}

func validateOptionalExpression(expression ast.Expression, ctx semantic.Context, narrowed map[graph.ResourceID]bool) error {
	if call, ok := expression.(*ast.CallExpression); ok {
		if name, yes := call.Callee.(*ast.IdentifierExpression); yes {
			if name.Name == "present" {
				return nil
			}
			if name.Name == "both" {
				active := cloneNarrowed(narrowed)
				for _, argument := range call.Arguments {
					if err := validateOptionalExpression(argument, ctx, active); err != nil {
						return err
					}
					for _, id := range definitelyPresent(argument) {
						active[id] = true
					}
				}
				return nil
			}
		}
		if err := validateOptionalExpression(call.Callee, ctx, narrowed); err != nil {
			return err
		}
		for _, argument := range call.Arguments {
			if err := validateOptionalExpression(argument, ctx, narrowed); err != nil {
				return err
			}
		}
		return nil
	}
	if binary, ok := expression.(*ast.BinaryExpression); ok && binary.Operator == "&&" {
		if err := validateOptionalExpression(binary.Left, ctx, narrowed); err != nil {
			return err
		}
		active := cloneNarrowed(narrowed)
		for _, id := range definitelyPresent(binary.Left) {
			active[id] = true
		}
		return validateOptionalExpression(binary.Right, ctx, active)
	}
	if path, ok := semantic.Path(expression); ok && len(path) >= 3 {
		if ctx.PresentPath != nil {
			id := graph.ResourceID("application." + path[0] + "." + path[1] + "." + path[2])
			if _, optional := ctx.PresentPath(path[:3]); optional && !narrowed[id] {
				return coded("SEM046", "optional export %s requires a successful present() guard", id)
			}
		}
		return nil
	}
	switch typed := expression.(type) {
	case *ast.BinaryExpression:
		if err := validateOptionalExpression(typed.Left, ctx, narrowed); err != nil {
			return err
		}
		return validateOptionalExpression(typed.Right, ctx, narrowed)
	case *ast.UnaryExpression:
		return validateOptionalExpression(typed.Operand, ctx, narrowed)
	case *ast.ParenthesisedExpression:
		return validateOptionalExpression(typed.Expression, ctx, narrowed)
	case *ast.IndexExpression:
		if err := validateOptionalExpression(typed.Object, ctx, narrowed); err != nil {
			return err
		}
		return validateOptionalExpression(typed.Index, ctx, narrowed)
	case *ast.ListExpression:
		for _, element := range typed.Elements {
			if err := validateOptionalExpression(element, ctx, narrowed); err != nil {
				return err
			}
		}
	case *ast.ObjectExpression:
		for _, entry := range typed.Entries {
			if err := validateOptionalExpression(entry.Value, ctx, narrowed); err != nil {
				return err
			}
		}
	}
	return nil
}

func cloneNarrowed(input map[graph.ResourceID]bool) map[graph.ResourceID]bool {
	out := map[graph.ResourceID]bool{}
	for id, present := range input {
		out[id] = present
	}
	return out
}

func definitelyPresent(expression ast.Expression) []graph.ResourceID {
	call, ok := expression.(*ast.CallExpression)
	if ok {
		if name, yes := call.Callee.(*ast.IdentifierExpression); yes {
			if name.Name == "present" && len(call.Arguments) == 1 {
				if path, valid := semantic.Path(call.Arguments[0]); valid && len(path) == 3 {
					return []graph.ResourceID{graph.ResourceID("application." + path[0] + "." + path[1] + "." + path[2])}
				}
			}
			if name.Name == "both" {
				var out []graph.ResourceID
				for _, argument := range call.Arguments {
					out = append(out, definitelyPresent(argument)...)
				}
				return out
			}
		}
	}
	if binary, ok := expression.(*ast.BinaryExpression); ok && binary.Operator == "&&" {
		return append(definitelyPresent(binary.Left), definitelyPresent(binary.Right)...)
	}
	return nil
}

func validateExternalMutationTarget(g *graph.Graph, exports map[graph.ResourceID]module.Export, narrowed map[graph.ResourceID]bool, statement ast.Statement) error {
	operation, ok := op(statement)
	if !ok {
		return nil
	}
	targetExpression := operation.Target
	if targetExpression == nil && operation.Operation == "enable" {
		for _, item := range operation.Body {
			assignment, yes := item.(*ast.AssignmentStatement)
			if !yes {
				continue
			}
			path, named := semantic.Path(assignment.Target)
			if named && len(path) == 1 && path[0] == "target" {
				targetExpression = assignment.Value
				break
			}
		}
	}
	if targetExpression == nil {
		return nil
	}
	path, ok := semantic.Path(targetExpression)
	if !ok || len(path) < 3 {
		return nil
	}
	id := graph.ResourceID("application." + path[0] + "." + path[1] + "." + path[2])
	if exported, known := exports[id]; known {
		if exported.Optional && !narrowed[id] {
			return coded("SEM046", "optional export %s requires a successful present() guard", id)
		}
		if !exported.Present {
			return coded("SEM046", "optional export %s is absent", id)
		}
		return nil
	}
	if resource, found := g.Get(id); found && resource.Metadata.Module != "" && !resource.Metadata.Exported {
		return coded("SEM049", "resource %s is private to its module", id)
	}
	return nil
}

func target(e ast.Expression) (graph.ResourceID, graph.FieldPath, error) {
	p, ok := semantic.Path(e)
	if !ok || len(p) < 3 {
		return "", nil, fmt.Errorf("invalid resource target")
	}
	return graph.ResourceID("application." + p[0] + "." + p[1] + "." + p[2]), graph.FieldPath(p[3:]), nil
}
func (c *Compiler) applyStatement(g *graph.Graph, ps *provenance.Store, s ast.Statement, owner provenance.Owner, explicit bool) ([]transform.FieldWrite, error) {
	o, ok := op(s)
	if !ok {
		return nil, nil
	}
	if o.Operation == "enable" {
		if o.Name != capability.Autoscaling && o.Name != capability.DisruptionProtection {
			return nil, fmt.Errorf("unknown built-in capability `%s`", o.Name)
		}
		body, e := statementsObject(o.Body, semantic.Context{ResolvePath: func(p []string) (value.Value, bool) {
			id, path, err := targetPath(p)
			if err != nil {
				return value.Value{}, false
			}
			return value.Reference(value.ReferenceValue{Target: string(id), Field: path}), true
		}})
		if e != nil {
			return nil, e
		}
		alias := ""
		if t, ok := body.Get("target"); ok {
			if ref, yes := t.ReferenceValue(); yes {
				parts := strings.Split(ref.Target, ".")
				if len(parts) > 1 {
					alias = parts[1]
				}
			}
		}
		if alias == "" {
			return nil, fmt.Errorf("capability requires target")
		}
		id := graph.ResourceID("application." + alias + ".capability." + o.Name)
		typ := graph.TypeName("core.Autoscaling")
		resourceName := alias + "-autoscaling"
		if o.Name == capability.DisruptionProtection {
			typ = "core.DisruptionProtection"
			resourceName = alias + "-disruption-protection"
		}
		if t, ok := body.Get("target"); ok {
			if ref, yes := t.ReferenceValue(); yes {
				if targetRes, found := g.Get(graph.ResourceID(ref.Target)); found {
					suffix := "autoscaling"
					if o.Name == capability.DisruptionProtection {
						suffix = "disruption-protection"
					}
					resourceName = targetRes.Name + "-" + suffix
				}
			}
		}
		r := graph.Resource{ID: id, Type: typ, Name: resourceName, Fields: body, Metadata: graph.Metadata{Source: s.Span(), Exported: true}}
		if e := g.Add(r); e != nil {
			return nil, e
		}
		ps.Add(provenance.Event{ResourceID: id, Action: provenance.CapabilityEnabled, Current: body, Owner: owner, Source: s.Span()})
		return nil, nil
	}
	if o.Operation == "add" {
		id, _, e := target(o.Target)
		if e != nil {
			return nil, e
		}
		body, e := statementsObject(o.Body, semantic.Context{})
		if e != nil {
			return nil, e
		}
		if o.Identity != "" {
			body, _ = body.With("name", value.String(o.Identity))
		}
		field := o.Name + "s"
		if o.Name == "port" {
			field = "ports"
		}
		path := graph.FieldPath{field}
		if err := validateMutationPoint(g, id, path); err != nil {
			return nil, err
		}
		cur, _ := g.ReadField(id, path)
		list, _ := cur.ListValue()
		list = append(list, body)
		next := value.List(list)
		if e = g.SetField(id, path, next); e != nil {
			return nil, e
		}
		ps.Add(provenance.Event{ResourceID: id, FieldPath: path, Action: provenance.TransformApplied, Current: next, Owner: owner, Source: s.Span()})
		return []transform.FieldWrite{{ResourceID: id, Path: path, Operation: transform.Add, Value: body, Owner: owner, Source: s.Span()}}, nil
	}
	id, path, e := target(o.Target)
	if e != nil {
		return nil, e
	}
	if owner.Kind == "variant" || owner.Kind == "transform" {
		if err := validateMutationPoint(g, id, path); err != nil {
			return nil, err
		}
	}
	prev, _ := g.ReadField(id, path)
	var next value.Value
	if o.Value != nil {
		next, e = semantic.Evaluate(o.Value, semantic.Context{})
		if e != nil {
			return nil, e
		}
	} else if len(o.Body) > 0 {
		next, e = statementsObject(o.Body, semantic.Context{})
		if e != nil {
			return nil, e
		}
	}
	kind := transform.OperationKind(o.Operation)
	switch o.Operation {
	case "delete":
		e = g.DeleteField(id, path)
	case "append":
		cur, _ := g.ReadField(id, path)
		a, _ := cur.ListValue()
		b, _ := next.ListValue()
		a = append(a, b...)
		next = value.List(a)
		e = g.SetField(id, path, next)
	case "merge":
		cur, _ := g.ReadField(id, path)
		cm, _ := cur.ObjectValue()
		nm, _ := next.ObjectValue()
		if cm == nil {
			cm = map[string]value.Value{}
		}
		for k, v := range nm {
			cm[k] = v
		}
		next = value.Object(cm)
		e = g.SetField(id, path, next)
	default:
		e = g.SetField(id, path, next)
	}
	if e != nil {
		return nil, e
	}
	action := provenance.TransformApplied
	if owner.Kind == "variant" {
		action = provenance.VariantSet
	}
	if explicit {
		action = provenance.ConflictResolved
	}
	ps.Add(provenance.Event{ResourceID: id, FieldPath: path, Action: action, Previous: prev, Current: next, Owner: owner, Source: s.Span()})
	if o.Operation == "merge" {
		if m, ok := next.ObjectValue(); ok {
			out := make([]transform.FieldWrite, 0, len(m))
			for _, k := range next.Keys() {
				out = append(out, transform.FieldWrite{ResourceID: id, Path: append(path.Clone(), k), Operation: kind, Value: m[k], Owner: owner, Source: s.Span(), Explicit: explicit})
			}
			return out, nil
		}
	}
	return []transform.FieldWrite{{ResourceID: id, Path: path, Operation: kind, Value: next, Owner: owner, Source: s.Span(), Explicit: explicit}}, nil
}

func validateMutationPoint(g *graph.Graph, id graph.ResourceID, path graph.FieldPath) error {
	resource, ok := g.Get(id)
	if !ok {
		return fmt.Errorf("resource not found: %s", id)
	}
	for _, protected := range resource.Metadata.Protected {
		if pathsOverlap(path, protected) {
			return fmt.Errorf("protected field modification: %s.%s", id, path.String())
		}
	}
	for _, extension := range resource.Metadata.Extensions {
		if pathHasPrefix(path, extension) {
			return nil
		}
	}
	return fmt.Errorf("field is not an extension point: %s.%s", id, path.String())
}

func pathsOverlap(a, b graph.FieldPath) bool {
	return pathHasPrefix(a, b) || pathHasPrefix(b, a)
}

func pathHasPrefix(path, prefix graph.FieldPath) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

func (c *Compiler) resolveReferences(g *graph.Graph, ds *diagnostics.List) {
	for _, r := range g.List() {
		walkReferences(r.Fields, func(ref value.ReferenceValue) {
			target := graph.ResourceID(ref.Target)
			tr, ok := g.Get(target)
			if !ok {
				*ds = append(*ds, diag("SEM020", "dangling reference to `"+ref.Target+"`", ref.Source))
				return
			}
			if len(ref.Field) > 0 {
				if _, ok := g.ReadField(target, graph.FieldPath(ref.Field)); !ok {
					*ds = append(*ds, diag("SEM021", "referenced field does not exist", ref.Source))
					return
				}
			}
			_ = tr
			_ = g.AddReference(r.ID, target)
		})
	}
}
func walkReferences(v value.Value, fn func(value.ReferenceValue)) {
	if r, ok := v.ReferenceValue(); ok {
		fn(r)
		return
	}
	if a, ok := v.ListValue(); ok {
		for _, x := range a {
			walkReferences(x, fn)
		}
	}
	if m, ok := v.ObjectValue(); ok {
		for _, x := range m {
			walkReferences(x, fn)
		}
	}
}
func targetPath(p []string) (graph.ResourceID, graph.FieldPath, error) {
	if len(p) < 3 {
		return "", nil, fmt.Errorf("invalid target")
	}
	return graph.ResourceID("application." + p[0] + "." + p[1] + "." + p[2]), graph.FieldPath(p[3:]), nil
}
func conflict(ws []transform.FieldWrite) bool {
	if len(ws) < 2 {
		return false
	}
	owners := map[string]value.Value{}
	for _, w := range ws {
		if v, ok := owners[w.Owner.Name]; ok && !v.Equal(w.Value) {
			return true
		}
		owners[w.Owner.Name] = w.Value
	}
	if len(owners) < 2 {
		return false
	}
	var first *value.Value
	for _, v := range owners {
		x := v
		if first == nil {
			first = &x
		} else if !first.Equal(v) {
			return true
		}
	}
	return false
}
func (c *Compiler) policies(pg *program, e *ast.EnvironmentDeclaration, options policy.Options, g *graph.Graph, ps *provenance.Store, ds *diagnostics.List) policy.Report {
	selected := map[string]bool{}
	for _, s := range e.Body {
		if b, ok := s.(*ast.BlockDeclaration); ok && b.Name == "policies" {
			for _, q := range b.Body {
				if u, yes := q.(*ast.UseStatement); yes {
					selected[u.Name] = true
				}
			}
		}
	}
	if len(selected) == 0 {
		for n := range pg.policies {
			selected[n] = true
		}
	}
	if len(options.Include) > 0 {
		selected = make(map[string]bool, len(options.Include))
		for _, name := range options.Include {
			selected[name] = true
		}
	}
	for _, name := range options.Exclude {
		delete(selected, name)
	}
	var report policy.Report
	count := 0
	names := make([]string, 0, len(selected))
	for n := range selected {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		pd := pg.policies[n]
		if pd == nil {
			*ds = append(*ds, diag("POL001", "unknown policy `"+n+"`", e.Span()))
			continue
		}
		downgradeAllowed := policyDowngradeAllowed(pd)
		for _, st := range pd.Body {
			sel, ok := st.(*ast.SelectStatement)
			if !ok {
				continue
			}
			for _, r := range g.List() {
				if sel.Type != "Resource" && coreTypeName(sel.Type) != r.Type {
					continue
				}
				count++
				if count > c.limits.MaxPolicyEvaluations {
					*ds = append(*ds, diag("POL010", "policy evaluation limit exceeded", pd.Span()))
					return report
				}
				vals, _ := r.Fields.ObjectValue()
				ctx := semantic.Context{Values: vals}
				if sel.Where != nil {
					v, er := semantic.Evaluate(sel.Where, ctx)
					if er != nil {
						continue
					}
					b, _ := v.BoolValue()
					if !b {
						continue
					}
				}
				for _, rule := range sel.Body {
					var cond ast.Expression
					var rt policy.RuleType
					var body []ast.Statement
					switch x := rule.(type) {
					case *ast.RequireStatement:
						cond, body, rt = x.Condition, x.Body, policy.Require
					case *ast.DenyStatement:
						y := ast.RequireStatement(*x)
						cond, body, rt = y.Condition, y.Body, policy.Deny
					case *ast.WarnStatement:
						y := ast.RequireStatement(*x)
						cond, body, rt = y.Condition, y.Body, policy.Warn
					default:
						continue
					}
					v, er := semantic.Evaluate(cond, ctx)
					if er != nil {
						continue
					}
					b, _ := v.BoolValue()
					violated := (rt == policy.Deny && b) || (rt != policy.Deny && !b)
					if !violated {
						continue
					}
					msg := "policy rule failed"
					mv, _ := statementsObject(body, ctx)
					if x, ok := mv.Get("message"); ok {
						if s, yes := x.StringValue(); yes {
							msg = s
						}
					}
					sev := diagnostics.SeverityError
					if rt == policy.Warn {
						sev = diagnostics.SeverityWarning
					} else if options.FailureMode == policy.FailureModeWarn && downgradeAllowed {
						sev = diagnostics.SeverityWarning
					}
					report.Results = append(report.Results, policy.Result{Policy: n, Rule: rt, Severity: sev, ResourceID: r.ID, Message: msg, PolicySource: rule.Span(), ResourceSource: r.Metadata.Source, DowngradeAllowed: downgradeAllowed})
					*ds = append(*ds, diagnostics.Diagnostic{Code: "POL002", Severity: sev, Message: msg, Span: rule.Span(), Related: []diagnostics.Related{{Message: string(r.ID), Span: r.Metadata.Source}}})
					ps.Add(provenance.Event{ResourceID: r.ID, Action: provenance.PolicyValidated, Owner: provenance.Owner{Kind: "policy", Name: n}, Source: rule.Span()})
				}
			}
		}
	}
	report.Sort()
	return report
}

func policyDowngradeAllowed(pd *ast.PolicyDeclaration) bool {
	for _, statement := range pd.Body {
		assignment, ok := statement.(*ast.AssignmentStatement)
		if !ok {
			continue
		}
		path, ok := semantic.Path(assignment.Target)
		if !ok || len(path) != 1 || path[0] != "downgradeAllowed" {
			continue
		}
		v, err := semantic.Evaluate(assignment.Value, semantic.Context{})
		if err != nil {
			return false
		}
		allowed, _ := v.BoolValue()
		return allowed
	}
	return false
}
func coreTypeName(s string) graph.TypeName {
	if strings.HasPrefix(s, "core.") {
		return graph.TypeName(s)
	}
	return graph.TypeName("core." + s)
}

type codedError struct {
	code    string
	message string
}

func (e codedError) Error() string { return e.message }

func coded(code, format string, arguments ...any) error {
	return codedError{code: code, message: fmt.Sprintf(format, arguments...)}
}

func errorCode(err error, fallback string) string {
	var typed codedError
	if errors.As(err, &typed) {
		return typed.code
	}
	return fallback
}

func conditionError(err error) error {
	if strings.Contains(err.Error(), "unknown function") {
		return coded("SEM044", "%s", err)
	}
	return coded("SEM045", "%s", err)
}

func diag(code, msg string, s diagnostics.Span) diagnostics.Diagnostic {
	return diagnostics.Diagnostic{Code: code, Severity: diagnostics.SeverityError, Message: msg, Span: s}
}
func sourceDigest(input Input) string {
	h := sha256.New()
	var size [8]byte
	files := append([]source.File(nil), input.RootProject.Files...)
	for _, pkg := range input.Packages {
		for _, f := range pkg.Files {
			f.Name = "package/" + pkg.Identity.String() + "@" + pkg.Version.String() + "/" + f.Name
			files = append(files, f)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	for _, f := range files {
		binary.BigEndian.PutUint64(size[:], uint64(len(f.Name)))
		h.Write(size[:])
		h.Write([]byte(f.Name))
		binary.BigEndian.PutUint64(size[:], uint64(len(f.Content)))
		h.Write(size[:])
		h.Write(f.Content)
	}
	writeStrings := func(values []string) {
		binary.BigEndian.PutUint64(size[:], uint64(len(values)))
		h.Write(size[:])
		for _, item := range values {
			binary.BigEndian.PutUint64(size[:], uint64(len(item)))
			h.Write(size[:])
			h.Write([]byte(item))
		}
	}
	writeStrings(input.Variants)
	writeStrings(input.Policy.Include)
	writeStrings(input.Policy.Exclude)
	writeStrings([]string{string(input.Policy.FailureMode)})
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
