package lockfile

import (
	"bytes"
	"testing"
)

func TestDeterministicSerialization(t *testing.T) {
	digest := "sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	f := File{Project: "p", DependencyDigest: digest, Packages: []Package{
		{Identity: "z/pkg", Version: "1.0.0", Source: "path:z", ManifestDigest: digest, ContentDigest: digest},
		{Identity: "a/pkg", Version: "2.0.0", Source: "path:a", Aliases: []string{"z", "a"}, ManifestDigest: digest, ContentDigest: digest},
	}}
	a, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("serialization changed")
	}
	parsed, ds := Parse(a, "mosaic.lock")
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	if parsed.Packages[0].Identity != "a/pkg" || parsed.Packages[0].Aliases[0] != "a" {
		t.Fatalf("not sorted: %#v", parsed)
	}
}
