// Package lockfile reads and writes deterministic Mosaic dependency locks.
package lockfile

import "sort"

const FormatVersion = "v1alpha1"

type File struct {
	FormatVersion    string    `toml:"format_version" json:"formatVersion"`
	Project          string    `toml:"project" json:"project"`
	DependencyDigest string    `toml:"dependency_digest" json:"dependencyDigest"`
	Packages         []Package `toml:"package" json:"packages"`
}

type Package struct {
	Identity          string   `toml:"identity" json:"identity"`
	Version           string   `toml:"version" json:"version"`
	Source            string   `toml:"source" json:"source"`
	DeclaredSource    string   `toml:"declared_source,omitempty" json:"declaredSource,omitempty"`
	Aliases           []string `toml:"aliases,omitempty" json:"aliases,omitempty"`
	ManifestDigest    string   `toml:"manifest_digest" json:"manifestDigest"`
	ContentDigest     string   `toml:"content_digest" json:"contentDigest"`
	ArchiveDigest     string   `toml:"archive_digest,omitempty" json:"archiveDigest,omitempty"`
	OCIManifestDigest string   `toml:"oci_manifest_digest,omitempty" json:"ociManifestDigest,omitempty"`
	Dependencies      []string `toml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Replacement       bool     `toml:"replacement,omitempty" json:"replacement,omitempty"`
}

func (f File) Sorted() File {
	out := f
	out.Packages = append([]Package(nil), f.Packages...)
	for i := range out.Packages {
		out.Packages[i].Aliases = sorted(out.Packages[i].Aliases)
		out.Packages[i].Dependencies = sorted(out.Packages[i].Dependencies)
	}
	sort.Slice(out.Packages, func(i, j int) bool {
		if out.Packages[i].Identity != out.Packages[j].Identity {
			return out.Packages[i].Identity < out.Packages[j].Identity
		}
		return out.Packages[i].Version < out.Packages[j].Version
	})
	return out
}

func (f File) Lookup(identity string) (Package, bool) {
	for _, pkg := range f.Packages {
		if pkg.Identity == identity {
			return pkg, true
		}
	}
	return Package{}, false
}

func sorted(in []string) []string { out := append([]string(nil), in...); sort.Strings(out); return out }
