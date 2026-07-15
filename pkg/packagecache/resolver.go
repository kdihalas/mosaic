package packagecache

import (
	"context"
	"strings"

	"github.com/kdihalas/mosaic/pkg/dependency"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
)

// Resolver exposes verified locked cache entries to offline dependency solving.
type Resolver struct {
	cache  *Cache
	locked *lockfile.File
}

func NewResolver(cache *Cache, locked *lockfile.File) *Resolver {
	return &Resolver{cache: cache, locked: locked}
}
func (r *Resolver) Supports(ref dependency.SourceReference) bool {
	return r != nil && r.locked != nil && strings.HasPrefix(ref.Source, "oci://")
}
func (r *Resolver) ListVersions(ctx context.Context, ref dependency.SourceReference) ([]dependency.VersionMetadata, diagnostics.List) {
	var out []dependency.VersionMetadata
	for _, pkg := range r.locked.Packages {
		if pkg.Source != ref.Source {
			continue
		}
		artifact, ok := r.cache.Get(ctx, pkg)
		if !ok {
			continue
		}
		version, _ := mosaicpackage.ParseVersion(pkg.Version)
		out = append(out, dependency.VersionMetadata{Version: version, Manifest: artifact.Manifest, Source: pkg.Source, ManifestDigest: pkg.ManifestDigest, ContentDigest: pkg.ContentDigest, ArchiveDigest: pkg.ArchiveDigest, OCIManifestDigest: pkg.OCIManifestDigest})
	}
	if len(out) == 0 {
		return nil, cacheDiagnostic("DEP041", "package metadata is unavailable offline", ref.Source)
	}
	return out, nil
}
func (r *Resolver) FetchManifest(ctx context.Context, ref dependency.ExactPackageReference) (*mosaicpackage.Manifest, diagnostics.List) {
	for _, pkg := range r.locked.Packages {
		if pkg.Identity == ref.Identity.String() && pkg.Version == ref.Version.String() {
			loaded, ds := r.cache.LoadPackage(ctx, pkg)
			if ds.HasErrors() {
				return nil, ds
			}
			manifest := loaded.Manifest
			return &manifest, nil
		}
	}
	return nil, cacheDiagnostic("DEP041", "package is unavailable offline", ref.Source)
}
func (r *Resolver) FetchPackage(ctx context.Context, ref dependency.ExactPackageReference) (*mosaicpackage.Package, diagnostics.List) {
	for _, pkg := range r.locked.Packages {
		if pkg.Identity == ref.Identity.String() && pkg.Version == ref.Version.String() {
			return r.cache.LoadPackage(ctx, pkg)
		}
	}
	return nil, cacheDiagnostic("DEP041", "package is unavailable offline", ref.Source)
}
