package packagecache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kdihalas/mosaic/pkg/lockfile"
	"github.com/kdihalas/mosaic/pkg/packagearchive"
	"github.com/kdihalas/mosaic/pkg/packagecache"
	mosaicvendor "github.com/kdihalas/mosaic/pkg/vendor"
)

func TestCacheAndVendorIntegrity(t *testing.T) {
	ctx := context.Background()
	root := packageRoot(t)
	artifact, ds := packagearchive.Pack(ctx, root, packagearchive.PackOptions{})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	lockedPackage := lockfile.Package{Identity: artifact.Manifest.Name, Version: artifact.Manifest.Version, Source: "path:package", ManifestDigest: artifact.ManifestDigest, ContentDigest: artifact.ContentDigest, ArchiveDigest: artifact.ArchiveDigest}
	file := lockfile.File{FormatVersion: lockfile.FormatVersion, Project: "p", DependencyDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Packages: []lockfile.Package{lockedPackage}}
	cache, err := packagecache.New(packagecache.Options{Root: filepath.Join(t.TempDir(), "cache")})
	if err != nil {
		t.Fatal(err)
	}
	if _, ds := cache.Put(ctx, artifact.Bytes, lockedPackage); ds.HasErrors() {
		t.Fatal(ds)
	}
	if _, ok := cache.Get(ctx, lockedPackage); !ok {
		t.Fatal("cache miss")
	}
	vendorRoot := filepath.Join(t.TempDir(), "vendor")
	if _, ds := mosaicvendor.Write(ctx, file, cache, mosaicvendor.Options{Output: vendorRoot}); ds.HasErrors() {
		t.Fatal(ds)
	}
	if ds := mosaicvendor.Verify(ctx, vendorRoot, file); ds.HasErrors() {
		t.Fatal(ds)
	}
	sourcePath := filepath.Join(vendorRoot, "packages", "acme", "cache-test", "1.0.0", "src", "module.mosaic")
	if err := os.WriteFile(sourcePath, []byte("module Changed {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ds := mosaicvendor.Verify(ctx, vendorRoot, file); !ds.HasErrors() || ds[0].Code != "LOCK003" {
		t.Fatalf("expected modified vendor diagnostic: %v", ds)
	}
}

func packageRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(name, data string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mosaic.package.toml", "name = \"acme/cache-test\"\nversion = \"1.0.0\"\nlanguage_version = \"v1alpha1\"\nsources = [\"src/**/*.mosaic\"]\n[exports]\nmodules = [\"CacheTest\"]\n")
	write("src/module.mosaic", "module CacheTest {}\n")
	return root
}
