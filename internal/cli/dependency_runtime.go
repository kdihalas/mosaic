package cli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/dependency"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/packagecache"
	"github.com/kdihalas/mosaic/pkg/packagefs"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/registry/oci"
	mosaicvendor "github.com/kdihalas/mosaic/pkg/vendor"
)

type dependencyRunOptions struct{ locked, offline, vendor, updateLock bool }

func (c *config) cache() (*packagecache.Cache, error) {
	return packagecache.New(packagecache.Options{Root: c.cacheDir})
}

func (c *config) ociClient(offline bool) (*oci.Client, error) {
	if offline {
		return nil, nil
	}
	credentials, err := oci.NewDockerCredentials()
	if err != nil {
		return nil, err
	}
	return oci.New(oci.Options{Credentials: credentials, PlainHTTP: c.plainHTTP}), nil
}

func (c *config) resolveProject(ctx context.Context, p *project.Project, existing *lockfile.File, options dependency.ResolveOptions) (*dependency.Resolution, error) {
	local := packagefs.NewLocalResolver(packagefs.Options{ProjectRoot: p.Root, AllowExternal: p.Config.AllowExternalLocalDependencies})
	sources := []dependency.SourceResolver{local}
	if options.Offline {
		cache, err := c.cache()
		if err != nil {
			return nil, err
		}
		if existing != nil {
			sources = append(sources, packagecache.NewResolver(cache, existing))
		}
	} else {
		remote, err := c.ociClient(false)
		if err != nil {
			return nil, err
		}
		sources = append(sources, remote)
	}
	resolution, ds := dependency.NewResolver(sources...).Resolve(ctx, dependency.ResolveInput{Project: p.Config, ProjectRoot: p.Root, ExistingLock: existing, Options: options})
	if len(ds) > 0 {
		c.printDiagnostics(ds)
	}
	if ds.HasErrors() {
		return nil, exitError{1, errors.New("dependency resolution failed")}
	}
	return resolution, nil
}

func (c *config) loadCompilation(ctx context.Context, options dependencyRunOptions) (*project.Project, []compiler.CompilationPackage, error) {
	if options.locked && options.updateLock {
		return nil, nil, exitError{2, errors.New("--locked and --update-lock cannot be combined")}
	}
	p, e := c.load(ctx)
	if e != nil {
		return nil, nil, e
	}
	if len(p.Config.Dependencies) == 0 {
		return p, nil, nil
	}
	lockPath := filepath.Join(p.Root, "mosaic.lock")
	locked, ds := lockfile.Read(lockPath)
	if options.updateLock {
		if ds.HasErrors() {
			locked = nil
		}
		resolution, err := c.resolveProject(ctx, p, locked, dependency.ResolveOptions{Offline: options.offline})
		if err != nil {
			return nil, nil, err
		}
		next := resolution.Lock(p.Config)
		if err := lockfile.Write(lockPath, next); err != nil {
			return nil, nil, exitError{3, err}
		}
		locked = &next
	} else {
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return nil, nil, exitError{1, errors.New("dependency lockfile is required")}
		}
		if stale := lockfile.ValidateProject(*locked, p.Config, lockPath); stale.HasErrors() {
			c.printDiagnostics(stale)
			return nil, nil, exitError{1, errors.New("dependency lockfile is stale")}
		}
	}
	cache, err := c.cache()
	if err != nil {
		return nil, nil, exitError{3, err}
	}
	local := packagefs.NewLocalResolver(packagefs.Options{ProjectRoot: p.Root, AllowExternal: p.Config.AllowExternalLocalDependencies})
	sources := []packagecache.ArchiveSource{local}
	if !options.offline && !options.vendor {
		remote, err := c.ociClient(false)
		if err != nil {
			return nil, nil, exitError{3, err}
		}
		sources = append(sources, remote)
	}
	if options.vendor {
		vendorRoot := filepath.Join(p.Root, "vendor", "mosaic")
		if vds := mosaicvendor.Verify(ctx, vendorRoot, *locked); vds.HasErrors() {
			c.printDiagnostics(vds)
			return nil, nil, exitError{1, errors.New("vendored dependencies failed verification")}
		}
	} else {
		_, rds := packagecache.NewRestorer(cache, packagecache.RestoreOptions{Offline: options.offline, Sources: sources}).Restore(ctx, *locked)
		if rds.HasErrors() {
			c.printDiagnostics(rds)
			return nil, nil, exitError{1, errors.New("dependency restore failed")}
		}
	}
	var packages []compiler.CompilationPackage
	for _, item := range locked.Packages {
		var pkg *mosaicpackage.Package
		if options.vendor {
			pkg, ds = mosaicvendor.LoadPackage(ctx, filepath.Join(p.Root, "vendor", "mosaic"), item)
		} else if strings.HasPrefix(item.Source, "path:") {
			identity, _ := mosaicpackage.ParseIdentity(item.Identity)
			version, _ := mosaicpackage.ParseVersion(item.Version)
			pkg, ds = local.FetchPackage(ctx, dependency.ExactPackageReference{SourceReference: dependency.SourceReference{Path: strings.TrimPrefix(item.Source, "path:"), DeclaringRoot: p.Root}, Identity: identity, Version: version, Digest: item.ContentDigest})
		} else {
			pkg, ds = cache.LoadPackage(ctx, item)
		}
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return nil, nil, exitError{1, errors.New("dependency source loading failed")}
		}
		identity, _ := mosaicpackage.ParseIdentity(item.Identity)
		version, _ := mosaicpackage.ParseVersion(item.Version)
		cp := compiler.CompilationPackage{Identity: identity, Version: version, Aliases: append([]string(nil), item.Aliases...), Root: pkg.Root, Files: pkg.Files, Manifest: pkg.Manifest, Exports: pkg.Manifest.Exports}
		for _, depID := range item.Dependencies {
			if at := strings.LastIndex(depID, "@"); at > 0 {
				depIdentity, _ := mosaicpackage.ParseIdentity(depID[:at])
				depVersion, _ := mosaicpackage.ParseVersion(depID[at+1:])
				cp.Dependencies = append(cp.Dependencies, compiler.PackageDependency{Identity: depIdentity, Version: depVersion})
			}
		}
		packages = append(packages, cp)
	}
	return p, packages, nil
}
