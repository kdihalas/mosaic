package lockfile

import (
	"fmt"
	"testing"
)

func BenchmarkParseLargeLockfile(b *testing.B) {
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	file := File{FormatVersion: FormatVersion, Project: "bench", DependencyDigest: digest}
	for i := 0; i < 500; i++ {
		file.Packages = append(file.Packages, Package{Identity: fmt.Sprintf("bench/p%03d", i), Version: "1.0.0", Source: "oci://bench/p", ManifestDigest: digest, ContentDigest: digest})
	}
	data, _ := Marshal(file)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ds := Parse(data, "mosaic.lock"); ds.HasErrors() {
			b.Fatal(ds)
		}
	}
}
