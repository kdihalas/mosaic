package mosaicpackage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/semantic"
	"github.com/kdihalas/mosaic/pkg/syntax/ast"
	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/parser"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

// Load validates and loads a package's selected Mosaic sources.
func Load(ctx context.Context, root string) (*Package, diagnostics.List) {
	m, ds := LoadManifest(ctx, root)
	if ds.HasErrors() {
		return nil, ds
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, manifestDiagnostic("PKG001", err.Error(), root)
	}
	var names []string
	err = filepath.WalkDir(abs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not permitted: %s", rel)
		}
		if entry.IsDir() {
			if excluded(m.Exclude, rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if selected(m.Sources, rel) && !excluded(m.Exclude, rel) {
			names = append(names, rel)
		}
		return nil
	})
	if err != nil {
		return nil, manifestDiagnostic("PKG010", err.Error(), root)
	}
	sort.Strings(names)
	pkg := &Package{Root: abs, Manifest: m}
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(abs, filepath.FromSlash(name)))
		if err != nil {
			return nil, manifestDiagnostic("PKG001", err.Error(), name)
		}
		pkg.Files = append(pkg.Files, source.NewFile(name, b))
	}
	if len(pkg.Files) == 0 {
		ds = append(ds, manifestDiagnostic("PKG001", "package source patterns matched no files", ManifestName)...)
	}
	ds = append(ds, ValidateExports(pkg)...)
	if ds.HasErrors() {
		return pkg, ds.Sorted()
	}
	return pkg, ds.Sorted()
}

func selected(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if ok, _ := doublestar.Match(pattern, name); ok {
			return true
		}
	}
	return false
}

func excluded(patterns []string, name string) bool {
	return selected(patterns, strings.TrimSuffix(name, "/")) || selected(patterns, name)
}

// ValidateExports parses source and validates the explicit public surface.
func ValidateExports(pkg *Package) diagnostics.List {
	if pkg == nil {
		return manifestDiagnostic("PKG001", "package is nil", ManifestName)
	}
	type declaration struct {
		category string
		node     ast.Declaration
	}
	decls := map[string]declaration{}
	var ds diagnostics.List
	for _, file := range pkg.Files {
		lr := lexer.Lex(file, lexer.Options{})
		pr := parser.Parse(file, lr.Tokens, parser.Options{})
		ds = append(ds, lr.Diagnostics...)
		ds = append(ds, pr.Diagnostics...)
		for _, node := range pr.File.Declarations {
			category, name := declarationName(node)
			if name == "" {
				continue
			}
			if previous, exists := decls[name]; exists {
				ds = append(ds, diagnostics.Diagnostic{Code: "SEM001", Severity: diagnostics.SeverityError, Message: fmt.Sprintf("duplicate declaration %q (%s and %s)", name, previous.category, category), Span: node.Span()})
				continue
			}
			decls[name] = declaration{category: category, node: node}
		}
	}
	for category, names := range pkg.Manifest.Exports.Categories() {
		for _, name := range names {
			d, ok := decls[name]
			if !ok {
				ds = append(ds, manifestDiagnostic("PKG020", fmt.Sprintf("exported %s %q does not exist", category, name), ManifestName)...)
				continue
			}
			if d.category != category {
				ds = append(ds, manifestDiagnostic("PKG020", fmt.Sprintf("symbol %q is a %s, not a %s", name, d.category, category), ManifestName)...)
			}
		}
	}
	publicTypes := map[string]bool{}
	for _, name := range pkg.Manifest.Exports.Types {
		publicTypes[name] = true
	}
	for _, name := range pkg.Manifest.Exports.Enums {
		publicTypes[name] = true
	}
	builtins := builtinTypes()
	var checkType func(owner string, expr ast.Expression, stack map[string]bool)
	checkType = func(owner string, expr ast.Expression, stack map[string]bool) {
		path, ok := semantic.Path(expr)
		if !ok || len(path) == 0 {
			if call, yes := expr.(*ast.CallExpression); yes {
				for _, argument := range call.Arguments {
					checkType(owner, argument, stack)
				}
			}
			return
		}
		name := path[len(path)-1]
		if len(path) > 1 || builtins[name] {
			return
		}
		d, exists := decls[name]
		if !exists || d.category != "type" && d.category != "enum" {
			return
		}
		if !publicTypes[name] {
			ds = append(ds, diagnostics.Diagnostic{Code: "PKG021", Severity: diagnostics.SeverityError, Message: fmt.Sprintf("exported %s exposes private type %q", owner, name), Span: expr.Span(), Suggestion: "export the type or change the public interface"})
			return
		}
		if stack[name] {
			return
		}
		stack[name] = true
		if typ, yes := d.node.(*ast.TypeDeclaration); yes {
			for _, field := range typ.Fields {
				checkType(owner, field.Type, stack)
			}
		}
		delete(stack, name)
	}
	for _, name := range pkg.Manifest.Exports.Modules {
		if d, ok := decls[name]; ok {
			if module, yes := d.node.(*ast.ModuleDeclaration); yes && module.Parameter != nil {
				checkType("module "+name, module.Parameter.Type, map[string]bool{})
			}
		}
	}
	for _, name := range pkg.Manifest.Exports.Capabilities {
		if d, ok := decls[name]; ok {
			if capability, yes := d.node.(*ast.CapabilityDeclaration); yes && capability.Parameter != nil {
				checkType("capability "+name, capability.Parameter.Type, map[string]bool{})
			}
		}
	}
	for _, name := range pkg.Manifest.Exports.Types {
		if d, ok := decls[name]; ok {
			if typ, yes := d.node.(*ast.TypeDeclaration); yes {
				for _, field := range typ.Fields {
					checkType("type "+name, field.Type, map[string]bool{name: true})
				}
			}
		}
	}
	return ds.Sorted()
}

func declarationName(node ast.Declaration) (string, string) {
	switch x := node.(type) {
	case *ast.ModuleDeclaration:
		return "module", x.Name
	case *ast.CapabilityDeclaration:
		return "capability", x.Name
	case *ast.TypeDeclaration:
		return "type", x.Name
	case *ast.EnumDeclaration:
		return "enum", x.Name
	case *ast.VariantDeclaration:
		return "variant", x.Name
	case *ast.TransformDeclaration:
		return "transform", x.Name
	case *ast.PolicyDeclaration:
		return "policy", x.Name
	case *ast.EnvironmentDeclaration:
		return "environment", x.Name
	case *ast.TestDeclaration:
		return "test", x.Name
	default:
		return "", ""
	}
}

func builtinTypes() map[string]bool {
	return map[string]bool{
		"null": true, "bool": true, "int": true, "decimal": true, "string": true,
		"list": true, "map": true, "optional": true, "resource": true, "reference": true,
		"ImageReference": true, "CpuQuantity": true, "MemoryQuantity": true, "Duration": true,
		"PortNumber": true, "ResourceName": true,
	}
}

// ValidateManifest validates fields that do not require parsing package source.
func ValidateManifest(m Manifest) diagnostics.List {
	var ds diagnostics.List
	add := func(code, message string) {
		ds = append(ds, manifestDiagnostic(code, message, ManifestName)...)
	}
	if _, err := ParseIdentity(m.Name); err != nil {
		add("PKG002", err.Error())
	}
	if _, err := ParseVersion(m.Version); err != nil {
		add("PKG003", err.Error())
	}
	if m.LanguageVersion != "v1alpha1" {
		add("PKG030", fmt.Sprintf("unsupported package language version %q", m.LanguageVersion))
	}
	if len(m.Sources) == 0 {
		add("PKG001", "package sources must not be empty")
	}
	for _, pattern := range append(append([]string(nil), m.Sources...), m.Exclude...) {
		if filepath.IsAbs(pattern) || strings.Contains(pattern, `\\`) || strings.Contains(pattern, "..") {
			add("PKG010", fmt.Sprintf("unsafe source pattern %q", pattern))
			continue
		}
		if _, err := doublestar.Match(pattern, "probe"); err != nil {
			add("PKG001", fmt.Sprintf("invalid source pattern %q: %v", pattern, err))
		}
	}
	seenExports := map[string]string{}
	for category, names := range m.Exports.Categories() {
		for _, name := range names {
			if name == "" {
				add("PKG020", "export name must not be empty")
				continue
			}
			if previous, ok := seenExports[name]; ok {
				add("PKG020", fmt.Sprintf("export %q is ambiguous between %s and %s", name, previous, category))
			} else {
				seenExports[name] = category
			}
		}
	}
	if len(m.Exports.Schemas) > 0 {
		add("PKG040", "schema declarations are reserved but not supported by language v1alpha1")
	}
	aliases := make([]string, 0, len(m.Dependencies))
	for alias := range m.Dependencies {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		d := m.Dependencies[alias]
		if err := validateAlias(alias); err != nil {
			add("DEP001", err.Error())
		}
		local := d.Path != ""
		remote := d.Source != "" || d.Version != ""
		if local == remote {
			add("PKG001", fmt.Sprintf("dependency %q must specify either path or source and version", alias))
			continue
		}
		if local {
			if filepath.IsAbs(d.Path) {
				add("PKG010", fmt.Sprintf("dependency %q path must be relative", alias))
			}
			continue
		}
		if !strings.HasPrefix(d.Source, "oci://") {
			add("OCI001", fmt.Sprintf("dependency %q has unsupported source %q", alias, d.Source))
		}
		if _, err := semver.NewConstraint(d.Version); err != nil {
			add("DEP002", fmt.Sprintf("dependency %q has invalid constraint: %v", alias, err))
		}
	}
	return ds.Sorted()
}
