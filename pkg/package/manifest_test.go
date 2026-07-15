package mosaicpackage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIdentityAndVersion(t *testing.T) {
	for _, valid := range []string{"acme/http-service", "community/postgres", "one", "acme.platform/http_v2"} {
		if _, err := ParseIdentity(valid); err != nil {
			t.Errorf("ParseIdentity(%q): %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "/acme", "acme/", "Acme/http", "acme//http", "acme/../http", "oci://host/repo", "acme/http@1.0.0", `acme\http`} {
		if _, err := ParseIdentity(invalid); err == nil {
			t.Errorf("ParseIdentity(%q) succeeded", invalid)
		}
	}
	if _, err := ParseVersion("1.4.0"); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"v1.4.0", "1.4", "^1.4.0", ""} {
		if _, err := ParseVersion(invalid); err == nil {
			t.Errorf("ParseVersion(%q) succeeded", invalid)
		}
	}
}

func TestLoadAndExportClosure(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ManifestName), `
name = "acme/http-service"
version = "1.4.0"
language_version = "v1alpha1"
sources = ["src/**/*.mosaic"]

[exports]
modules = ["HttpService"]
types = ["HttpServiceInput"]
`)
	mustWrite(t, filepath.Join(root, "src", "package.mosaic"), `
type InternalProfile { memory: string }
type HttpServiceInput { profile: InternalProfile }
module HttpService(input: HttpServiceInput) {}
`)
	pkg, ds := Load(context.Background(), root)
	if pkg == nil || !ds.HasErrors() {
		t.Fatalf("expected private type diagnostic, pkg=%v ds=%v", pkg, ds)
	}
	if ds[len(ds)-1].Code != "PKG021" {
		t.Fatalf("expected PKG021, got %#v", ds)
	}
}

func TestCanonicalManifestDigestIgnoresSetOrdering(t *testing.T) {
	a := Manifest{Name: "acme/a", Version: "1.0.0", LanguageVersion: "v1alpha1", Sources: []string{"b", "a"}, Authors: []string{"z", "a"}}
	b := a
	b.Sources = []string{"a", "b"}
	b.Authors = []string{"a", "z"}
	da, err := ManifestDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	db, err := ManifestDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	if da != db {
		t.Fatalf("digest changed: %s != %s", da, db)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
