// Package packagefs resolves local Mosaic packages within an explicit filesystem boundary.
package packagefs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kdihalas/mosaic/pkg/dependency"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/packagearchive"
)

type Options struct {
	ProjectRoot   string
	AllowExternal bool
	Limits        mosaicpackage.Limits
}

type Resolver struct{ options Options }

func NewLocalResolver(options Options) *Resolver                 { return &Resolver{options: options} }
func (r *Resolver) Supports(ref dependency.SourceReference) bool { return ref.Path != "" }

// SupportsSource reports whether the resolver can restore a locked source.
func (r *Resolver) SupportsLockedSource(source string) bool {
	return strings.HasPrefix(source, "path:")
}

// FetchArchive restores and verifies a locked local package archive.
func (r *Resolver) FetchArchive(ctx context.Context, locked lockfile.Package) ([]byte, diagnostics.List) {
	path := strings.TrimPrefix(locked.Source, "path:")
	root, ds := r.resolveRoot(dependency.SourceReference{Path: path, DeclaringRoot: r.options.ProjectRoot})
	if ds.HasErrors() {
		return nil, ds
	}
	artifact, ds := packagearchive.Pack(ctx, root, packagearchive.PackOptions{Limits: r.options.Limits})
	if ds.HasErrors() {
		return nil, ds
	}
	if artifact.Manifest.Name != locked.Identity || artifact.Manifest.Version != locked.Version || artifact.ContentDigest != locked.ContentDigest {
		return nil, localDiagnostic("LOCK003", "local package content does not match lockfile", locked.Source)
	}
	return artifact.Bytes, nil
}

func (r *Resolver) ListVersions(ctx context.Context, ref dependency.SourceReference) ([]dependency.VersionMetadata, diagnostics.List) {
	root, ds := r.resolveRoot(ref)
	if ds.HasErrors() {
		return nil, ds
	}
	artifact, ds := packagearchive.Pack(ctx, root, packagearchive.PackOptions{Limits: r.options.Limits})
	if ds.HasErrors() {
		return nil, ds
	}
	version, _ := mosaicpackage.ParseVersion(artifact.Manifest.Version)
	return []dependency.VersionMetadata{{Version: version, Manifest: artifact.Manifest, Source: r.lockSource(root), Root: root, ManifestDigest: artifact.ManifestDigest, ContentDigest: artifact.ContentDigest, ArchiveDigest: artifact.ArchiveDigest}}, nil
}

func (r *Resolver) FetchManifest(ctx context.Context, ref dependency.ExactPackageReference) (*mosaicpackage.Manifest, diagnostics.List) {
	versions, ds := r.ListVersions(ctx, ref.SourceReference)
	if ds.HasErrors() {
		return nil, ds
	}
	if len(versions) != 1 || versions[0].Version != ref.Version {
		return nil, localDiagnostic("DEP010", "local package version does not match exact reference", ref.Path)
	}
	manifest := versions[0].Manifest
	return &manifest, nil
}

func (r *Resolver) FetchPackage(ctx context.Context, ref dependency.ExactPackageReference) (*mosaicpackage.Package, diagnostics.List) {
	root, ds := r.resolveRoot(ref.SourceReference)
	if ds.HasErrors() {
		return nil, ds
	}
	pkg, ds := mosaicpackage.Load(ctx, root)
	if ds.HasErrors() {
		return pkg, ds
	}
	identity, _ := mosaicpackage.ParseIdentity(pkg.Manifest.Name)
	version, _ := mosaicpackage.ParseVersion(pkg.Manifest.Version)
	if identity != ref.Identity || version != ref.Version {
		return nil, localDiagnostic("DEP020", fmt.Sprintf("local package declares %s@%s", identity, version), ref.Path)
	}
	if ref.Digest != "" {
		artifact, more := packagearchive.Pack(ctx, root, packagearchive.PackOptions{Limits: r.options.Limits})
		if more.HasErrors() {
			return nil, more
		}
		if artifact.ContentDigest != ref.Digest {
			return nil, localDiagnostic("LOCK003", "local package content does not match lockfile", ref.Path)
		}
	}
	return pkg, ds
}

func (r *Resolver) resolveRoot(ref dependency.SourceReference) (string, diagnostics.List) {
	base := ref.DeclaringRoot
	if base == "" {
		base = r.options.ProjectRoot
	}
	if base == "" {
		return "", localDiagnostic("PKG010", "local resolver requires a project root", ref.Path)
	}
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", localDiagnostic("PKG010", err.Error(), ref.Path)
	}
	candidate := ref.Path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(baseAbs, candidate)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", localDiagnostic("DEP010", err.Error(), ref.Path)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", localDiagnostic("PKG010", err.Error(), ref.Path)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", localDiagnostic("DEP010", "local dependency is not a directory", ref.Path)
	}
	if !r.options.AllowExternal {
		projectRoot, err := filepath.EvalSymlinks(r.options.ProjectRoot)
		if err != nil {
			return "", localDiagnostic("PKG010", err.Error(), ref.Path)
		}
		rel, err := filepath.Rel(projectRoot, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return "", localDiagnostic("PKG010", "local dependency escapes the project root", ref.Path)
		}
	}
	return resolved, nil
}

func (r *Resolver) lockSource(root string) string {
	rel, err := filepath.Rel(r.options.ProjectRoot, root)
	if err == nil && !filepath.IsAbs(rel) {
		return "path:" + filepath.ToSlash(rel)
	}
	return "path:" + filepath.ToSlash(root)
}
func localDiagnostic(code, message, source string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: message, Span: diagnostics.Span{SourceName: source}}}
}
