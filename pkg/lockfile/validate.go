package lockfile

import (
	"fmt"
	"strings"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/project"
)

func ValidateStructure(file File, source string) diagnostics.List {
	if file.FormatVersion != FormatVersion {
		return diagnostic("LOCK005", fmt.Sprintf("unsupported lockfile version %q", file.FormatVersion), source)
	}
	seen := map[string]bool{}
	var ds diagnostics.List
	for _, pkg := range file.Packages {
		if _, err := mosaicpackage.ParseIdentity(pkg.Identity); err != nil {
			ds = append(ds, diagnostic("LOCK005", err.Error(), source)...)
		}
		if _, err := mosaicpackage.ParseVersion(pkg.Version); err != nil {
			ds = append(ds, diagnostic("LOCK005", err.Error(), source)...)
		}
		if seen[pkg.Identity] {
			ds = append(ds, diagnostic("LOCK005", "duplicate locked package "+pkg.Identity, source)...)
		}
		seen[pkg.Identity] = true
		for _, digest := range []string{pkg.ManifestDigest, pkg.ContentDigest} {
			if !validDigest(digest) {
				ds = append(ds, diagnostic("LOCK003", "invalid or missing locked digest", source)...)
			}
		}
	}
	return ds.Sorted()
}

func ValidateProject(file File, cfg project.Config, source string) diagnostics.List {
	ds := ValidateStructure(file, source)
	if file.Project != cfg.Name || file.DependencyDigest != project.DependencyDigest(cfg) {
		ds = append(ds, diagnostic("LOCK002", "lockfile does not match project dependency declarations", source)...)
	}
	return ds.Sorted()
}

func validDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	for _, r := range strings.TrimPrefix(value, "sha256:") {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
