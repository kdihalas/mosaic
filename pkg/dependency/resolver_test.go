package dependency

import (
	"context"
	"testing"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/project"
)

type memorySource struct{ versions map[string][]VersionMetadata }

func (m memorySource) Supports(SourceReference) bool { return true }
func (m memorySource) ListVersions(_ context.Context, ref SourceReference) ([]VersionMetadata, diagnostics.List) {
	return append([]VersionMetadata(nil), m.versions[ref.Source]...), nil
}
func (m memorySource) FetchManifest(context.Context, ExactPackageReference) (*mosaicpackage.Manifest, diagnostics.List) {
	return nil, nil
}
func (m memorySource) FetchPackage(context.Context, ExactPackageReference) (*mosaicpackage.Package, diagnostics.List) {
	return nil, nil
}

func TestHighestCompatibleAndConflict(t *testing.T) {
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	meta := func(name, version string, deps map[string]mosaicpackage.Dependency) VersionMetadata {
		return VersionMetadata{Version: mosaicpackage.Version(version), Manifest: mosaicpackage.Manifest{Name: name, Version: version, LanguageVersion: "v1alpha1", Sources: []string{"src/**"}, Dependencies: deps}, Source: "oci://registry/" + name, ManifestDigest: digest, ContentDigest: digest}
	}
	source := memorySource{versions: map[string][]VersionMetadata{
		"oci://registry/acme/http": {meta("acme/http", "1.0.0", map[string]mosaicpackage.Dependency{"obs": {Source: "oci://registry/acme/obs", Version: ">=2.0.0 <3.0.0"}}), meta("acme/http", "1.2.0", map[string]mosaicpackage.Dependency{"obs": {Source: "oci://registry/acme/obs", Version: "^2.1.0"}})},
		"oci://registry/acme/obs":  {meta("acme/obs", "2.1.0", nil), meta("acme/obs", "2.4.0", nil), meta("acme/obs", "3.0.0", nil)},
	}}
	cfg := project.Config{Name: "root", LanguageVersion: "v1alpha1", Dependencies: map[string]mosaicpackage.Dependency{"http": {Source: "oci://registry/acme/http", Version: "^1.0.0"}, "obs": {Source: "oci://registry/acme/obs", Version: "~2.1.0"}}}
	resolution, ds := NewResolver(source).Resolve(context.Background(), ResolveInput{Project: cfg})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	if len(resolution.Packages) != 2 || resolution.Packages[0].Version != "1.2.0" || resolution.Packages[1].Version != "2.1.0" {
		t.Fatalf("unexpected resolution: %#v", resolution.Packages)
	}
	cfg.Dependencies["obs"] = mosaicpackage.Dependency{Source: "oci://registry/acme/obs", Version: "^3.0.0"}
	if _, ds := NewResolver(source).Resolve(context.Background(), ResolveInput{Project: cfg}); !ds.HasErrors() || ds[0].Code != "DEP012" {
		t.Fatalf("expected conflict: %v", ds)
	}
}

func TestPrereleaseRequiresOptIn(t *testing.T) {
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	m := func(v string) VersionMetadata {
		return VersionMetadata{Version: mosaicpackage.Version(v), Manifest: mosaicpackage.Manifest{Name: "acme/a", Version: v, LanguageVersion: "v1alpha1", Sources: []string{"src/**"}}, Source: "oci://r/a", ManifestDigest: digest, ContentDigest: digest}
	}
	s := memorySource{versions: map[string][]VersionMetadata{"oci://r/a": {m("1.1.0-beta.1"), m("1.0.0")}}}
	cfg := project.Config{Name: "p", Dependencies: map[string]mosaicpackage.Dependency{"a": {Source: "oci://r/a", Version: "^1.0.0"}}}
	r, ds := NewResolver(s).Resolve(context.Background(), ResolveInput{Project: cfg})
	if ds.HasErrors() || r.Packages[0].Version != "1.0.0" {
		t.Fatalf("stable selection: %#v %v", r, ds)
	}
}
