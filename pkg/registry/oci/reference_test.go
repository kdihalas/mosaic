package oci

import "testing"

func TestParseReference(t *testing.T) {
	for _, test := range []struct{ raw, registry, repository, reference string }{
		{"oci://ghcr.io/acme/http", "ghcr.io", "acme/http", ""},
		{"oci://localhost:5000/acme/http:1.2.3", "localhost:5000", "acme/http", "1.2.3"},
		{"oci://ghcr.io/acme/http@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "ghcr.io", "acme/http", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	} {
		ref, err := ParseReference(test.raw)
		if err != nil {
			t.Fatal(err)
		}
		if ref.Registry != test.registry || ref.Repository != test.repository || ref.Reference != test.reference {
			t.Fatalf("%s: %#v", test.raw, ref)
		}
	}
	if _, err := ParseReference("https://ghcr.io/acme/http"); err == nil {
		t.Fatal("accepted non-OCI reference")
	}
}
