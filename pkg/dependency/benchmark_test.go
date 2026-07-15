package dependency

import (
	"context"
	"fmt"
	"testing"

	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/project"
)

func BenchmarkResolve500Packages(b *testing.B) {
	const count = 500
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	versions := map[string][]VersionMetadata{}
	deps := map[string]mosaicpackage.Dependency{}
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("bench/p%03d", i)
		source := "oci://bench/" + name
		versions[source] = []VersionMetadata{{Version: "1.0.0", Manifest: mosaicpackage.Manifest{Name: name, Version: "1.0.0", LanguageVersion: "v1alpha1", Sources: []string{"src/**"}}, Source: source, ManifestDigest: digest, ContentDigest: digest}}
		deps[fmt.Sprintf("p%03d", i)] = mosaicpackage.Dependency{Source: source, Version: "1.x"}
	}
	input := ResolveInput{Project: project.Config{Name: "bench", Dependencies: deps}}
	resolver := NewResolver(memorySource{versions: versions})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ds := resolver.Resolve(context.Background(), input); ds.HasErrors() {
			b.Fatal(ds)
		}
	}
}
