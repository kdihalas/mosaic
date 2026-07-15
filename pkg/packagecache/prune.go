package packagecache

import (
	"context"
	"os"
	"sort"
)

type PruneOptions struct {
	DryRun           bool
	ProtectedDigests map[string]bool
}

func (c *Cache) Prune(options PruneOptions) ([]Entry, error) {
	entries := c.List(context.Background())
	var removed []Entry
	for _, entry := range entries {
		if options.ProtectedDigests[entry.Metadata.ContentDigest] {
			continue
		}
		removed = append(removed, entry)
		if !options.DryRun {
			if err := os.RemoveAll(entry.Path); err != nil {
				return removed, err
			}
		}
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i].Path < removed[j].Path })
	return removed, nil
}
