// Package vendor creates and verifies deterministic project-local dependency trees.
package vendor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/packagearchive"
	"github.com/kdihalas/mosaic/pkg/packagecache"
)

const LockName = "vendor.lock"

type Options struct{ Output string }
type Report struct {
	Packages int    `json:"packages"`
	Output   string `json:"output"`
}

func Write(ctx context.Context, file lockfile.File, cache *packagecache.Cache, options Options) (Report, diagnostics.List) {
	output := options.Output
	if output == "" {
		output = filepath.Join("vendor", "mosaic")
	}
	abs, err := filepath.Abs(output)
	if err != nil {
		return Report{}, vendorDiagnostic("CACHE002", err.Error(), output)
	}
	parent := filepath.Dir(abs)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Report{}, vendorDiagnostic("CACHE002", err.Error(), parent)
	}
	temp, err := os.MkdirTemp(parent, ".mosaic-vendor-")
	if err != nil {
		return Report{}, vendorDiagnostic("CACHE002", err.Error(), parent)
	}
	defer func() { _ = os.RemoveAll(temp) }()
	for _, pkg := range file.Sorted().Packages {
		artifact, ok := cache.Get(ctx, pkg)
		if !ok {
			return Report{}, vendorDiagnostic("DEP041", "locked package is missing from verified cache: "+pkg.Identity+"@"+pkg.Version, pkg.Source)
		}
		destination := filepath.Join(temp, "packages", filepath.FromSlash(pkg.Identity), pkg.Version)
		identityVersion := packagearchive.VerifyOptions{ExpectedArchiveDigest: pkg.ArchiveDigest, ExpectedContentDigest: pkg.ContentDigest}
		if _, ds := packagearchive.Unpack(ctx, artifact.Bytes, destination, packagearchive.UnpackOptions{VerifyOptions: identityVersion}); ds.HasErrors() {
			return Report{}, ds
		}
	}
	lockBytes, err := lockfile.Marshal(file)
	if err != nil {
		return Report{}, vendorDiagnostic("CACHE002", err.Error(), LockName)
	}
	if err := os.WriteFile(filepath.Join(temp, LockName), lockBytes, 0o644); err != nil {
		return Report{}, vendorDiagnostic("CACHE002", err.Error(), LockName)
	}
	backup := abs + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(abs); err == nil {
		if err := os.Rename(abs, backup); err != nil {
			return Report{}, vendorDiagnostic("CACHE002", err.Error(), abs)
		}
	}
	if err := os.Rename(temp, abs); err != nil {
		_ = os.Rename(backup, abs)
		return Report{}, vendorDiagnostic("CACHE002", err.Error(), abs)
	}
	_ = os.RemoveAll(backup)
	return Report{Packages: len(file.Packages), Output: abs}, nil
}

func Verify(ctx context.Context, root string, file lockfile.File) diagnostics.List {
	lockedPath := filepath.Join(root, LockName)
	vendorLock, ds := lockfile.Read(lockedPath)
	if ds.HasErrors() {
		return ds
	}
	expected, _ := lockfile.Marshal(file)
	actual, _ := lockfile.Marshal(*vendorLock)
	if string(expected) != string(actual) {
		return vendorDiagnostic("LOCK002", "vendor.lock does not match mosaic.lock", lockedPath)
	}
	for _, pkg := range file.Packages {
		packageRoot := filepath.Join(root, "packages", filepath.FromSlash(pkg.Identity), pkg.Version)
		if _, err := os.Stat(packageRoot); err != nil {
			return vendorDiagnostic("DEP041", "vendored package is missing", packageRoot)
		}
		data, err := repack(ctx, packageRoot)
		if err != nil {
			return vendorDiagnostic("LOCK003", "vendored package is invalid or changed: "+err.Error(), packageRoot)
		}
		if data.ContentDigest != pkg.ContentDigest {
			return vendorDiagnostic("LOCK003", "vendored package content has changed", packageRoot)
		}
	}
	return nil
}

// LoadPackage verifies and loads one vendored package.
func LoadPackage(ctx context.Context, root string, pkg lockfile.Package) (*mosaicpackage.Package, diagnostics.List) {
	packageRoot := filepath.Join(root, "packages", filepath.FromSlash(pkg.Identity), pkg.Version)
	artifact, err := repack(ctx, packageRoot)
	if err != nil {
		return nil, vendorDiagnostic("CACHE001", err.Error(), packageRoot)
	}
	if artifact.ContentDigest != pkg.ContentDigest {
		return nil, vendorDiagnostic("LOCK003", "vendored package content has changed", packageRoot)
	}
	loaded, ds := mosaicpackage.Load(ctx, packageRoot)
	if ds.HasErrors() {
		return nil, ds
	}
	return loaded, ds
}

func repack(ctx context.Context, root string) (*packagearchive.Artifact, error) {
	artifact, ds := packagearchive.Pack(ctx, root, packagearchive.PackOptions{})
	if ds.HasErrors() {
		return nil, fmt.Errorf("%s", ds[0].Message)
	}
	return artifact, nil
}
func vendorDiagnostic(code, message, source string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: message, Span: diagnostics.Span{SourceName: strings.TrimSpace(source)}}}
}
