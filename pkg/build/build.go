// Package build exposes Mosaic's deterministic, network-free build pipeline.
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kdihalas/mosaic/pkg/bundle"
	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/policy"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/renderer/kubernetes"
	mosaicvendor "github.com/kdihalas/mosaic/pkg/vendor"
)

type InputKind string

const (
	InputKindAuto    InputKind = "Auto"
	InputKindProject InputKind = "Project"
	InputKindPackage InputKind = "Package"
	InputKindBundle  InputKind = "Bundle"
)

// Input is a complete, local-only Mosaic build request.
type Input struct {
	RootPath    string
	InputKind   InputKind
	Environment string
	Variants    []string
	Policy      policy.Options
	Offline     bool
	Locked      bool
	Limits      compiler.Limits
}

// Result contains the compiler and deterministic rendering products.
type Result struct {
	InputKind   InputKind
	Compilation *compiler.Result
	Rendered    *renderer.ArtifactSet
	Bundle      *bundle.Bundle
}

// Run loads, compiles, evaluates, and renders a Mosaic input without network
// access or mutation of the source directory.
func Run(ctx context.Context, input Input) (*Result, diagnostics.List) {
	if !input.Offline || !input.Locked {
		return nil, diagnostic("BLD001", "build requires offline and locked mode", input.RootPath)
	}
	if input.RootPath == "" {
		return nil, diagnostic("BLD001", "build root is empty", input.RootPath)
	}
	kind, ds := DetectInputKind(input.RootPath, input.InputKind)
	if ds.HasErrors() {
		return nil, ds
	}
	if input.Environment == "" {
		return nil, diagnostic("BLD001", "environment is empty", input.RootPath)
	}
	if kind == InputKindBundle {
		return runBundle(ctx, input)
	}

	root, packages, ds := loadSource(ctx, input.RootPath, kind, input.Environment)
	if ds.HasErrors() {
		return nil, ds
	}
	c := compiler.New(compiler.NewOptions{Limits: input.Limits})
	compiled, cds := c.CompileInput(ctx, compiler.Input{
		RootProject: root, Packages: packages, Environment: input.Environment,
		Variants: append([]string(nil), input.Variants...), Policy: input.Policy,
	})
	ds = append(ds, cds...)
	if ds.HasErrors() || compiled == nil {
		return nil, ds.Sorted()
	}
	rendered, rds := kubernetes.New().Render(ctx, renderer.RenderInput{
		Environment: compiled.Environment, Graph: compiled.Graph,
		Provenance: compiled.Provenance, Options: compiled.Metadata.TargetOptions,
	})
	ds = append(ds, rds...)
	if ds.HasErrors() || rendered == nil {
		return nil, ds.Sorted()
	}
	built, err := bundle.Build(compiled, *rendered, bundle.BuildOptions{})
	if err != nil {
		return nil, append(ds, diagnostic("BLD004", err.Error(), input.RootPath)...).Sorted()
	}
	return &Result{InputKind: kind, Compilation: compiled, Rendered: rendered, Bundle: built}, ds.Sorted()
}

// DetectInputKind validates an explicit input kind or detects an unambiguous
// Mosaic layout.
func DetectInputKind(root string, requested InputKind) (InputKind, diagnostics.List) {
	markers := []struct {
		kind InputKind
		name string
	}{{InputKindBundle, "bundle.json"}, {InputKindProject, "mosaic.toml"}, {InputKindPackage, mosaicpackage.ManifestName}}
	var found []InputKind
	for _, marker := range markers {
		if info, err := os.Stat(filepath.Join(root, marker.name)); err == nil && !info.IsDir() {
			found = append(found, marker.kind)
		}
	}
	if requested != "" && requested != InputKindAuto {
		for _, kind := range found {
			if kind == requested {
				return requested, nil
			}
		}
		return "", diagnostic("BLD002", fmt.Sprintf("input does not contain the marker for %s", requested), root)
	}
	if len(found) == 0 {
		return "", diagnostic("BLD002", "no Mosaic input marker found", root)
	}
	if len(found) > 1 {
		return "", diagnostic("BLD002", "ambiguous Mosaic input layout", root)
	}
	return found[0], nil
}

func loadSource(ctx context.Context, rootPath string, kind InputKind, environment string) (*project.Project, []compiler.CompilationPackage, diagnostics.List) {
	switch kind {
	case InputKindProject:
		p, ds := project.Load(ctx, rootPath, project.LoadOptions{})
		if ds.HasErrors() {
			return nil, nil, ds
		}
		packages, pds := loadVendored(ctx, p)
		return p, packages, append(ds, pds...).Sorted()
	case InputKindPackage:
		pkg, ds := mosaicpackage.Load(ctx, rootPath)
		if ds.HasErrors() {
			return nil, nil, ds
		}
		if !contains(pkg.Manifest.Exports.Environments, environment) {
			return nil, nil, append(ds, diagnostic("BLD003", "package does not export environment `"+environment+"`", mosaicpackage.ManifestName)...).Sorted()
		}
		p := project.New(pkg.Root, pkg.Manifest.Name, pkg.Files)
		p.Config = project.Config{
			Name: pkg.Manifest.Name, LanguageVersion: pkg.Manifest.LanguageVersion,
			Dependencies: pkg.Manifest.Dependencies,
		}
		packages, pds := loadVendored(ctx, p)
		return p, packages, append(ds, pds...).Sorted()
	default:
		return nil, nil, diagnostic("BLD002", "unsupported source input kind", rootPath)
	}
}

func loadVendored(ctx context.Context, p *project.Project) ([]compiler.CompilationPackage, diagnostics.List) {
	if len(p.Config.Dependencies) == 0 {
		return nil, nil
	}
	lockPath := filepath.Join(p.Root, "mosaic.lock")
	locked, ds := lockfile.Read(lockPath)
	if ds.HasErrors() {
		return nil, ds
	}
	if stale := lockfile.ValidateProject(*locked, p.Config, lockPath); stale.HasErrors() {
		return nil, stale
	}
	vendorRoot := filepath.Join(p.Root, "vendor", "mosaic")
	if vds := mosaicvendor.Verify(ctx, vendorRoot, *locked); vds.HasErrors() {
		return nil, vds
	}
	packages := make([]compiler.CompilationPackage, 0, len(locked.Packages))
	for _, item := range locked.Sorted().Packages {
		pkg, pds := mosaicvendor.LoadPackage(ctx, vendorRoot, item)
		if pds.HasErrors() {
			return nil, pds
		}
		identity, _ := mosaicpackage.ParseIdentity(item.Identity)
		version, _ := mosaicpackage.ParseVersion(item.Version)
		cp := compiler.CompilationPackage{
			Identity: identity, Version: version, Aliases: append([]string(nil), item.Aliases...),
			Root: pkg.Root, Files: pkg.Files, Manifest: pkg.Manifest, Exports: pkg.Manifest.Exports,
		}
		for _, dependencyID := range item.Dependencies {
			at := strings.LastIndex(dependencyID, "@")
			if at <= 0 {
				continue
			}
			depIdentity, identityErr := mosaicpackage.ParseIdentity(dependencyID[:at])
			depVersion, versionErr := mosaicpackage.ParseVersion(dependencyID[at+1:])
			if identityErr == nil && versionErr == nil {
				cp.Dependencies = append(cp.Dependencies, compiler.PackageDependency{Identity: depIdentity, Version: depVersion})
			}
		}
		packages = append(packages, cp)
	}
	return packages, nil
}

func runBundle(ctx context.Context, input Input) (*Result, diagnostics.List) {
	b, ds := bundle.ReadDirectory(ctx, input.RootPath)
	if ds.HasErrors() {
		return nil, ds
	}
	if b.Manifest.Environment != input.Environment {
		return nil, diagnostic("BLD005", "bundle environment does not match requested environment", input.RootPath)
	}
	derivationRequested := len(input.Variants) > 0 || len(input.Policy.Include) > 0 || len(input.Policy.Exclude) > 0 || input.Policy.FailureMode == policy.FailureModeWarn
	if derivationRequested {
		if b.Recipe == nil {
			return nil, diagnostic("BLD005", "bundle does not contain a composable build recipe", input.RootPath)
		}
		compilerInput, err := b.Recipe.CompilerInput(input.Environment)
		if err != nil {
			return nil, diagnostic("BLD005", err.Error(), "build-recipe.json")
		}
		compilerInput.Variants = append([]string(nil), input.Variants...)
		compilerInput.Policy = input.Policy
		compiled, cds := compiler.New(compiler.NewOptions{Limits: input.Limits}).CompileInput(ctx, compilerInput)
		ds = append(ds, cds...)
		if ds.HasErrors() || compiled == nil {
			return nil, ds.Sorted()
		}
		rendered, rds := kubernetes.New().Render(ctx, renderer.RenderInput{
			Environment: compiled.Environment, Graph: compiled.Graph,
			Provenance: compiled.Provenance, Options: compiled.Metadata.TargetOptions,
		})
		ds = append(ds, rds...)
		if ds.HasErrors() || rendered == nil {
			return nil, ds.Sorted()
		}
		derived, err := bundle.Build(compiled, *rendered, bundle.BuildOptions{})
		if err != nil {
			return nil, append(ds, diagnostic("BLD004", err.Error(), input.RootPath)...).Sorted()
		}
		return &Result{InputKind: InputKindBundle, Compilation: compiled, Rendered: rendered, Bundle: derived}, ds.Sorted()
	}
	rendered, rds := kubernetes.New().Render(ctx, renderer.RenderInput{Environment: input.Environment, Graph: b.Graph})
	ds = append(ds, rds...)
	if ds.HasErrors() {
		return nil, ds.Sorted()
	}
	return &Result{InputKind: InputKindBundle, Rendered: rendered, Bundle: b}, ds.Sorted()
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func diagnostic(code, message, source string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: message, Span: diagnostics.Span{SourceName: source}}}
}
