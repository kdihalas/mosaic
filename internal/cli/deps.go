package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kdihalas/mosaic/pkg/dependency"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/packagecache"
	"github.com/kdihalas/mosaic/pkg/packagefs"
	"github.com/kdihalas/mosaic/pkg/project"
	mosaicvendor "github.com/kdihalas/mosaic/pkg/vendor"
	"github.com/spf13/cobra"
)

func (c *config) depsCmd() *cobra.Command {
	x := &cobra.Command{Use: "deps", Short: "Manage project package dependencies"}
	x.AddCommand(c.depsAddCmd(), c.depsRemoveCmd(), c.depsResolveCmd(), c.depsRestoreCmd(), c.depsUpdateCmd(), c.depsListCmd(), c.depsGraphCmd(), c.depsVendorCmd())
	return x
}

func (c *config) depsResolveCmd() *cobra.Command {
	var offline, includePrerelease bool
	x := &cobra.Command{Use: "resolve", Short: "Resolve dependencies and write mosaic.lock", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, e := c.load(cmd.Context())
		if e != nil {
			return e
		}
		var existing *lockfile.File
		if f, ds := lockfile.Read(filepath.Join(p.Root, "mosaic.lock")); !ds.HasErrors() {
			existing = f
		}
		resolution, err := c.resolveProject(cmd.Context(), p, existing, dependency.ResolveOptions{Offline: offline, PreferLocked: true, IncludePrerelease: includePrerelease})
		if err != nil {
			return err
		}
		locked := resolution.Lock(p.Config)
		if err := lockfile.Write(filepath.Join(p.Root, "mosaic.lock"), locked); err != nil {
			return exitError{3, err}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(resolution, "", "  ")
			writeln(c.s.out, string(b))
		} else if !c.quiet {
			writef(c.s.out, "Resolved %d packages.\nWrote: %s\n", len(resolution.Packages), filepath.Join(p.Root, "mosaic.lock"))
		}
		return nil
	}}
	x.Flags().BoolVar(&offline, "offline", false, "resolve without network access")
	x.Flags().BoolVar(&includePrerelease, "include-prerelease", false, "allow prerelease versions")
	return x
}

func (c *config) depsRestoreCmd() *cobra.Command {
	var offline, vendorMode bool
	x := &cobra.Command{Use: "restore", Short: "Restore exact locked dependencies", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, err := c.load(cmd.Context())
		if err != nil {
			return err
		}
		locked, ds := lockfile.Read(filepath.Join(p.Root, "mosaic.lock"))
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("lockfile is required")}
		}
		cache, err := c.cache()
		if err != nil {
			return exitError{3, err}
		}
		local := packagefs.NewLocalResolver(packagefs.Options{ProjectRoot: p.Root, AllowExternal: p.Config.AllowExternalLocalDependencies})
		sources := []packagecache.ArchiveSource{local}
		if !offline && !vendorMode {
			remote, e := c.ociClient(false)
			if e != nil {
				return e
			}
			sources = append(sources, remote)
		}
		report, rds := packagecache.NewRestorer(cache, packagecache.RestoreOptions{Offline: offline || vendorMode, Sources: sources}).Restore(cmd.Context(), *locked)
		if rds.HasErrors() {
			c.printDiagnostics(rds)
			return exitError{1, errors.New("restore failed")}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(report, "", "  ")
			writeln(c.s.out, string(b))
		} else if !c.quiet {
			writef(c.s.out, "Restored %d packages.\nFrom cache: %d\nDownloaded: %d\nVerified: %d\n", report.Restored, report.FromCache, report.Downloaded, report.Verified)
		}
		return nil
	}}
	x.Flags().BoolVar(&offline, "offline", false, "do not access registries")
	x.Flags().BoolVar(&vendorMode, "vendor", false, "use only vendored dependencies")
	return x
}

func (c *config) depsAddCmd() *cobra.Command {
	return &cobra.Command{Use: "add <alias> <source> [constraint]", Short: "Add and resolve a dependency transactionally", Args: cobra.RangeArgs(2, 3), RunE: func(cmd *cobra.Command, args []string) error {
		p, e := c.load(cmd.Context())
		if e != nil {
			return e
		}
		cfg := p.Config
		cfg.Dependencies = cloneDependencies(cfg.Dependencies)
		if _, exists := cfg.Dependencies[args[0]]; exists {
			return exitError{2, fmt.Errorf("dependency alias %s already exists", args[0])}
		}
		if strings.HasPrefix(args[1], "oci://") {
			if len(args) != 3 {
				return exitError{2, errors.New("OCI dependencies require a version constraint")}
			}
			cfg.Dependencies[args[0]] = mosaicpackage.Dependency{Source: args[1], Version: args[2]}
		} else {
			if len(args) != 2 {
				return exitError{2, errors.New("local dependencies do not accept a version constraint")}
			}
			cfg.Dependencies[args[0]] = mosaicpackage.Dependency{Path: filepath.ToSlash(args[1])}
		}
		candidate := *p
		candidate.Config = cfg
		resolution, err := c.resolveProject(cmd.Context(), &candidate, nil, dependency.ResolveOptions{})
		if err != nil {
			return err
		}
		if err := writeProjectAndLock(p.Root, cfg, resolution.Lock(cfg)); err != nil {
			return exitError{3, err}
		}
		if !c.quiet {
			writef(c.s.out, "Added dependency %s and updated mosaic.lock.\n", args[0])
		}
		return nil
	}}
}

func (c *config) depsRemoveCmd() *cobra.Command {
	return &cobra.Command{Use: "remove <alias>", Short: "Remove and re-resolve a dependency transactionally", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, e := c.load(cmd.Context())
		if e != nil {
			return e
		}
		cfg := p.Config
		cfg.Dependencies = cloneDependencies(cfg.Dependencies)
		if _, ok := cfg.Dependencies[args[0]]; !ok {
			return exitError{2, fmt.Errorf("unknown dependency alias %s", args[0])}
		}
		delete(cfg.Dependencies, args[0])
		candidate := *p
		candidate.Config = cfg
		resolution, err := c.resolveProject(cmd.Context(), &candidate, nil, dependency.ResolveOptions{})
		if err != nil {
			return err
		}
		if err := writeProjectAndLock(p.Root, cfg, resolution.Lock(cfg)); err != nil {
			return exitError{3, err}
		}
		if !c.quiet {
			writef(c.s.out, "Removed dependency %s and updated mosaic.lock.\n", args[0])
		}
		return nil
	}}
}

func (c *config) depsUpdateCmd() *cobra.Command {
	var dryRun, includePrerelease, major bool
	x := &cobra.Command{Use: "update [aliases...]", Short: "Safely update dependencies within declared constraints", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, e := c.load(cmd.Context())
		if e != nil {
			return e
		}
		existing, ds := lockfile.Read(filepath.Join(p.Root, "mosaic.lock"))
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("lockfile is required")}
		}
		if major {
			return exitError{2, errors.New("--major constraint widening is not yet supported")}
		}
		resolution, err := c.resolveProject(cmd.Context(), p, existing, dependency.ResolveOptions{IncludePrerelease: includePrerelease})
		if err != nil {
			return err
		}
		next := resolution.Lock(p.Config)
		changes := lockChanges(*existing, next, args)
		if c.format == "json" {
			b, _ := json.MarshalIndent(changes, "", "  ")
			writeln(c.s.out, string(b))
		} else {
			writeln(c.s.out, "Dependency update preview:")
			for _, change := range changes {
				writef(c.s.out, "%s  %s -> %s\n", change.Identity, change.From, change.To)
			}
		}
		if !dryRun {
			if err := lockfile.Write(filepath.Join(p.Root, "mosaic.lock"), next); err != nil {
				return exitError{3, err}
			}
		}
		return nil
	}}
	x.Flags().BoolVar(&dryRun, "dry-run", false, "preview without writing")
	x.Flags().BoolVar(&includePrerelease, "include-prerelease", false, "allow prerelease versions")
	x.Flags().BoolVar(&major, "major", false, "allow widening direct constraints")
	return x
}

type dependencyChange struct{ Identity, From, To string }

func lockChanges(before, after lockfile.File, aliases []string) []dependencyChange {
	selected := map[string]bool{}
	for _, a := range aliases {
		selected[a] = true
	}
	var out []dependencyChange
	for _, pkg := range after.Packages {
		if len(selected) > 0 {
			match := false
			for _, a := range pkg.Aliases {
				match = match || selected[a]
			}
			if !match {
				continue
			}
		}
		old, _ := before.Lookup(pkg.Identity)
		if old.Version != pkg.Version {
			out = append(out, dependencyChange{pkg.Identity, old.Version, pkg.Version})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Identity < out[j].Identity })
	return out
}

func (c *config) depsListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List direct and transitive dependencies", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, e := c.load(cmd.Context())
		if e != nil {
			return e
		}
		file, ds := lockfile.Read(filepath.Join(p.Root, "mosaic.lock"))
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("lockfile is required")}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(file.Packages, "", "  ")
			writeln(c.s.out, string(b))
			return nil
		}
		writeln(c.s.out, "DIRECT")
		for _, pkg := range file.Packages {
			if len(pkg.Aliases) > 0 {
				writef(c.s.out, "%s\n  %s@%s\n  %s\n  %s\n", strings.Join(pkg.Aliases, ", "), pkg.Identity, pkg.Version, pkg.Source, pkg.ContentDigest)
			}
		}
		writeln(c.s.out, "\nTRANSITIVE")
		for _, pkg := range file.Packages {
			if len(pkg.Aliases) == 0 {
				writef(c.s.out, "%s@%s\n", pkg.Identity, pkg.Version)
			}
		}
		return nil
	}}
}

func (c *config) depsGraphCmd() *cobra.Command {
	var format string
	x := &cobra.Command{Use: "graph", Short: "Show the locked dependency graph", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, e := c.load(cmd.Context())
		if e != nil {
			return e
		}
		file, ds := lockfile.Read(filepath.Join(p.Root, "mosaic.lock"))
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("lockfile is required")}
		}
		if format == "json" {
			b, _ := json.MarshalIndent(file, "", "  ")
			writeln(c.s.out, string(b))
			return nil
		}
		if format == "dot" {
			writeln(c.s.out, "digraph dependencies {")
			for _, pkg := range file.Packages {
				for _, dep := range pkg.Dependencies {
					writef(c.s.out, "  %q -> %q;\n", pkg.Identity+"@"+pkg.Version, dep)
				}
			}
			writeln(c.s.out, "}")
			return nil
		}
		writeln(c.s.out, p.Name)
		for _, pkg := range file.Packages {
			if len(pkg.Aliases) > 0 {
				writef(c.s.out, "├── %s@%s\n", pkg.Identity, pkg.Version)
				for _, dep := range pkg.Dependencies {
					writef(c.s.out, "│   └── %s\n", dep)
				}
			}
		}
		return nil
	}}
	x.Flags().StringVar(&format, "format", "tree", "tree, json, or dot")
	return x
}

func (c *config) depsVendorCmd() *cobra.Command {
	var output string
	var offline bool
	x := &cobra.Command{Use: "vendor", Short: "Vendor exact locked dependencies", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, _, e := c.loadCompilation(cmd.Context(), dependencyRunOptions{locked: true, offline: offline})
		if e != nil {
			return e
		}
		file, ds := lockfile.Read(filepath.Join(p.Root, "mosaic.lock"))
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("lockfile is required")}
		}
		cache, err := c.cache()
		if err != nil {
			return exitError{3, err}
		}
		if output == "" {
			output = filepath.Join(p.Root, "vendor", "mosaic")
		}
		report, vds := mosaicvendor.Write(cmd.Context(), *file, cache, mosaicvendor.Options{Output: output})
		if vds.HasErrors() {
			c.printDiagnostics(vds)
			return exitError{1, errors.New("vendoring failed")}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(report, "", "  ")
			writeln(c.s.out, string(b))
		} else if !c.quiet {
			writef(c.s.out, "Vendored %d packages to %s\n", report.Packages, report.Output)
		}
		return nil
	}}
	x.Flags().StringVar(&output, "output", "", "vendor output directory")
	x.Flags().BoolVar(&offline, "offline", false, "use verified cache only")
	return x
}

func cloneDependencies(in map[string]mosaicpackage.Dependency) map[string]mosaicpackage.Dependency {
	out := map[string]mosaicpackage.Dependency{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func writeProjectAndLock(root string, cfg project.Config, file lockfile.File) error {
	configBytes, err := project.MarshalConfig(cfg)
	if err != nil {
		return err
	}
	lockBytes, err := lockfile.Marshal(file)
	if err != nil {
		return err
	}
	configPath, lockPath := filepath.Join(root, "mosaic.toml"), filepath.Join(root, "mosaic.lock")
	configTemp, err := os.CreateTemp(root, ".mosaic-config-")
	if err != nil {
		return err
	}
	configName := configTemp.Name()
	defer func() { _ = os.Remove(configName) }()
	if _, err := configTemp.Write(configBytes); err != nil {
		_ = configTemp.Close()
		return err
	}
	if err := configTemp.Close(); err != nil {
		return err
	}
	lockTemp, err := os.CreateTemp(root, ".mosaic-lock-")
	if err != nil {
		return err
	}
	lockName := lockTemp.Name()
	defer func() { _ = os.Remove(lockName) }()
	if _, err := lockTemp.Write(lockBytes); err != nil {
		_ = lockTemp.Close()
		return err
	}
	if err := lockTemp.Close(); err != nil {
		return err
	}
	configBackup := configPath + ".previous"
	lockBackup := lockPath + ".previous"
	_ = os.Remove(configBackup)
	_ = os.Remove(lockBackup)
	if err := os.Rename(configPath, configBackup); err != nil {
		return err
	}
	if _, err := os.Stat(lockPath); err == nil {
		if err := os.Rename(lockPath, lockBackup); err != nil {
			_ = os.Rename(configBackup, configPath)
			return err
		}
	}
	if err := os.Rename(configName, configPath); err != nil {
		_ = os.Rename(configBackup, configPath)
		_ = os.Rename(lockBackup, lockPath)
		return err
	}
	if err := os.Rename(lockName, lockPath); err != nil {
		_ = os.Remove(configPath)
		_ = os.Rename(configBackup, configPath)
		_ = os.Rename(lockBackup, lockPath)
		return err
	}
	_ = os.Remove(configBackup)
	_ = os.Remove(lockBackup)
	return nil
}
