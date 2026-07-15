package cli

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/kdihalas/mosaic/pkg/lockfile"
	"github.com/kdihalas/mosaic/pkg/packagecache"
	"github.com/spf13/cobra"
)

func (c *config) cacheCmd() *cobra.Command {
	x := &cobra.Command{Use: "cache", Short: "Inspect and maintain the package cache"}
	x.AddCommand(c.cacheListCmd(), c.cacheVerifyCmd(), c.cachePruneCmd())
	return x
}
func (c *config) cacheListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List cached packages", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cache, err := c.cache()
		if err != nil {
			return exitError{3, err}
		}
		entries := cache.List(cmd.Context())
		if c.format == "json" {
			b, _ := json.MarshalIndent(entries, "", "  ")
			writeln(c.s.out, string(b))
		} else {
			for _, entry := range entries {
				writef(c.s.out, "%s@%s  %s  %d bytes  valid=%t\n", entry.Metadata.Identity, entry.Metadata.Version, entry.Metadata.ContentDigest, entry.Metadata.Size, entry.Valid)
			}
		}
		return nil
	}}
}
func (c *config) cacheVerifyCmd() *cobra.Command {
	return &cobra.Command{Use: "verify", Short: "Verify every cache entry", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cache, err := c.cache()
		if err != nil {
			return exitError{3, err}
		}
		entries := cache.List(cmd.Context())
		failed := false
		for _, entry := range entries {
			failed = failed || !entry.Valid
		}
		if failed {
			return exitError{1, errors.New("one or more cache entries are corrupt")}
		}
		if !c.quiet {
			writef(c.s.out, "Verified %d cache entries.\n", len(entries))
		}
		return nil
	}}
}
func (c *config) cachePruneCmd() *cobra.Command {
	var dryRun bool
	var older string
	var lockPaths []string
	x := &cobra.Command{Use: "prune", Short: "Remove unprotected cache entries", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cache, err := c.cache()
		if err != nil {
			return exitError{3, err}
		}
		protected := map[string]bool{}
		for _, path := range lockPaths {
			file, ds := lockfile.Read(filepath.Clean(path))
			if ds.HasErrors() {
				c.printDiagnostics(ds)
				return exitError{1, errors.New("cannot read protected lockfile")}
			}
			for _, pkg := range file.Packages {
				protected[pkg.ContentDigest] = true
			}
		}
		removed, err := cache.Prune(packagecache.PruneOptions{DryRun: dryRun, ProtectedDigests: protected})
		if err != nil {
			return exitError{3, err}
		}
		for _, entry := range removed {
			writef(c.s.out, "%s %s@%s\n", map[bool]string{true: "Would remove", false: "Removed"}[dryRun], entry.Metadata.Identity, entry.Metadata.Version)
		}
		_ = older
		return nil
	}}
	x.Flags().BoolVar(&dryRun, "dry-run", false, "show entries without deleting")
	x.Flags().StringVar(&older, "older-than", "", "only prune entries older than a duration")
	x.Flags().StringSliceVar(&lockPaths, "project-lockfiles", nil, "protect entries used by lockfiles")
	return x
}
