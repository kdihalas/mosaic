package packagearchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackIsDeterministicAndVerifiable(t *testing.T) {
	root := packageFixture(t)
	a, ds := Pack(context.Background(), root, PackOptions{})
	if ds.HasErrors() {
		t.Fatalf("first pack: %v", ds)
	}
	if err := os.Chtimes(filepath.Join(root, "src", "module.mosaic"), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	b, ds := Pack(context.Background(), root, PackOptions{})
	if ds.HasErrors() {
		t.Fatalf("second pack: %v", ds)
	}
	if !bytes.Equal(a.Bytes, b.Bytes) {
		t.Fatal("archives differ")
	}
	verified, ds := Verify(context.Background(), a.Bytes, VerifyOptions{ExpectedArchiveDigest: a.ArchiveDigest, ExpectedContentDigest: a.ContentDigest})
	if ds.HasErrors() || verified.Manifest.Name != "acme/example" {
		t.Fatalf("verify: %#v %v", verified, ds)
	}
}

func TestVerifyRejectsTraversalAndSymlink(t *testing.T) {
	malicious := rawArchive(t, &tar.Header{Name: "package/../secret", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg}, []byte("x"))
	if _, ds := Verify(context.Background(), malicious, VerifyOptions{}); !ds.HasErrors() || ds[0].Code != "ARCH001" {
		t.Fatalf("expected ARCH001: %v", ds)
	}
	symlink := rawArchive(t, &tar.Header{Name: "package/link", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "target"}, nil)
	if _, ds := Verify(context.Background(), symlink, VerifyOptions{}); !ds.HasErrors() || ds[0].Code != "ARCH002" {
		t.Fatalf("expected ARCH002: %v", ds)
	}
}

func packageFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(name, content string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mosaic.package.toml", "name = \"acme/example\"\nversion = \"1.0.0\"\nlanguage_version = \"v1alpha1\"\nsources = [\"src/**/*.mosaic\"]\n[exports]\nmodules = [\"Example\"]\n")
	write("src/module.mosaic", "module Example {}\n")
	write("README.md", "example\n")
	write("ignored.txt", "ignored\n")
	return root
}

func rawArchive(t *testing.T, header *tar.Header, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
