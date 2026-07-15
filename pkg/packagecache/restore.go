package packagecache

import (
	"context"
	"sort"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/lockfile"
)

type ArchiveSource interface {
	SupportsLockedSource(source string) bool
	FetchArchive(context.Context, lockfile.Package) ([]byte, diagnostics.List)
}
type RestoreOptions struct {
	Offline bool
	Sources []ArchiveSource
}
type RestoreReport struct {
	Restored   int `json:"restored"`
	FromCache  int `json:"fromCache"`
	Downloaded int `json:"downloaded"`
	Verified   int `json:"verified"`
}
type Restorer struct {
	cache   *Cache
	options RestoreOptions
}

func NewRestorer(cache *Cache, options RestoreOptions) *Restorer {
	return &Restorer{cache: cache, options: options}
}

func (r *Restorer) Restore(ctx context.Context, file lockfile.File) (RestoreReport, diagnostics.List) {
	packages := append([]lockfile.Package(nil), file.Packages...)
	sort.Slice(packages, func(i, j int) bool { return packages[i].Identity < packages[j].Identity })
	var report RestoreReport
	var ds diagnostics.List
	for _, pkg := range packages {
		if _, ok := r.cache.Get(ctx, pkg); ok {
			report.FromCache++
			report.Verified++
			report.Restored++
			continue
		}
		if r.options.Offline {
			ds = append(ds, cacheDiagnostic("DEP041", "package is unavailable offline: "+pkg.Identity+"@"+pkg.Version, pkg.Source)...)
			continue
		}
		var data []byte
		found := false
		for _, source := range r.options.Sources {
			if source.SupportsLockedSource(pkg.Source) {
				var more diagnostics.List
				data, more = source.FetchArchive(ctx, pkg)
				ds = append(ds, more...)
				found = !more.HasErrors()
				break
			}
		}
		if !found {
			if !ds.HasErrors() {
				ds = append(ds, cacheDiagnostic("DEP010", "no source can restore "+pkg.Identity, pkg.Source)...)
			}
			continue
		}
		if _, more := r.cache.Put(ctx, data, pkg); more.HasErrors() {
			ds = append(ds, more...)
			continue
		}
		report.Downloaded++
		report.Verified++
		report.Restored++
	}
	return report, ds.Sorted()
}
