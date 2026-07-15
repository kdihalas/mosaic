package oci

import (
	"fmt"
	"strings"

	orasregistry "oras.land/oras-go/v2/registry"
)

type Reference struct {
	Registry   string
	Repository string
	Reference  string
}

func ParseReference(raw string) (Reference, error) {
	value, ok := strings.CutPrefix(raw, "oci://")
	if !ok {
		return Reference{}, fmt.Errorf("OCI reference must start with oci://")
	}
	parsed, err := orasregistry.ParseReference(value)
	if err != nil {
		return Reference{}, err
	}
	return Reference{Registry: parsed.Registry, Repository: parsed.Repository, Reference: parsed.Reference}, nil
}
func (r Reference) RepositoryReference() string { return r.Registry + "/" + r.Repository }
func (r Reference) String() string {
	value := "oci://" + r.RepositoryReference()
	if r.Reference != "" {
		if strings.HasPrefix(r.Reference, "sha256:") {
			value += "@" + r.Reference
		} else {
			value += ":" + r.Reference
		}
	}
	return value
}
