package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/packagearchive"
	"github.com/kdihalas/mosaic/pkg/project"
	baseregistry "github.com/kdihalas/mosaic/pkg/registry"
	"github.com/kdihalas/mosaic/pkg/registry/oci"
	mtesting "github.com/kdihalas/mosaic/pkg/testing"
	"github.com/spf13/cobra"
)

func (c *config) packageCmd() *cobra.Command {
	x := &cobra.Command{Use: "package", Short: "Create, validate, pack, and distribute Mosaic packages"}
	x.AddCommand(c.packageInitCmd(), c.packageValidateCmd(), c.packagePackCmd(), c.packagePublishCmd(), c.packagePullCmd(), c.packageInspectCmd(), c.packageVerifyCmd())
	return x
}

func (c *config) packageInitCmd() *cobra.Command {
	return &cobra.Command{Use: "init [directory]", Short: "Create a complete Mosaic package", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		name := strings.ToLower(filepath.Base(filepath.Clean(dir)))
		files := packageScaffold(name)
		for rel, data := range files {
			path := filepath.Join(dir, filepath.FromSlash(rel))
			if _, err := os.Stat(path); err == nil {
				return exitError{3, fmt.Errorf("refusing to overwrite %s", path)}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return exitError{3, err}
			}
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				return exitError{3, err}
			}
		}
		if _, ds := mosaicpackage.Load(cmd.Context(), dir); ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("generated package failed validation")}
		}
		if !c.quiet {
			writef(c.s.out, "Initialised Mosaic package in %s\n", dir)
		}
		return nil
	}}
}

func packageScaffold(name string) map[string]string {
	return map[string]string{
		"mosaic.package.toml":        fmt.Sprintf("name = %q\nversion = \"0.1.0\"\ndescription = \"Reusable Mosaic package\"\nlanguage_version = \"v1alpha1\"\nlicense = \"Apache-2.0\"\nauthors = [\"Mosaic package authors\"]\nsources = [\"src/**/*.mosaic\", \"tests/**/*.mosaic\"]\nexclude = [\"dist/**\", \".git/**\"]\n\n[exports]\nmodules = [\"ExampleService\"]\ntypes = [\"ExampleServiceInput\"]\n", name),
		"src/types.mosaic":           "type ExampleServiceInput {\n    name: ResourceName\n    image: ImageReference\n}\n",
		"src/module.mosaic":          "module ExampleService(input: ExampleServiceInput) {\n    workload \"main\" {\n        name = input.name\n        image = input.image\n        replicas = 1\n    }\n}\n",
		"tests/package_tests.mosaic": "use ExampleService as example {\n    name = \"example\"\n    image = \"ghcr.io/example/service:1.0.0\"\n}\n\nenvironment packageTest {\n    use example\n    target kubernetes {\n        namespace = \"package-test\"\n    }\n}\n\ntest exampleBuilds {\n    build packageTest\n    assert example.workload.main.replicas == 1\n}\n",
		"README.md":                  "# " + name + "\n\nA reusable Mosaic package.\n",
		"LICENSE":                    "Apache License\nVersion 2.0, January 2004\nhttps://www.apache.org/licenses/LICENSE-2.0\n",
	}
}

func (c *config) validatePackage(ctx context.Context, path string, runTests bool) (*mosaicpackage.Package, error) {
	pkg, ds := mosaicpackage.Load(ctx, path)
	if len(ds) > 0 {
		c.printDiagnostics(ds)
	}
	if ds.HasErrors() {
		return pkg, exitError{1, errors.New("package validation failed")}
	}
	if runTests {
		p := project.New(pkg.Root, pkg.Manifest.Name, pkg.Files)
		report, tds := mtesting.Run(ctx, p, compiler.New(compiler.NewOptions{}))
		if len(tds) > 0 {
			c.printDiagnostics(tds)
		}
		if tds.HasErrors() || report.Failed > 0 {
			return pkg, exitError{1, errors.New("package tests failed")}
		}
	}
	return pkg, nil
}

func (c *config) packageValidateCmd() *cobra.Command {
	var offline, locked bool
	x := &cobra.Command{Use: "validate [path]", Short: "Validate a Mosaic package and its tests", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		pkg, err := c.validatePackage(cmd.Context(), path, true)
		if err != nil {
			return err
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(pkg.Manifest, "", "  ")
			writeln(c.s.out, string(b))
		} else if !c.quiet {
			writef(c.s.out, "Validated package %s@%s (%d source files).\n", pkg.Manifest.Name, pkg.Manifest.Version, len(pkg.Files))
		}
		_ = offline
		_ = locked
		return nil
	}}
	x.Flags().BoolVar(&offline, "offline", false, "do not access registries")
	x.Flags().BoolVar(&locked, "locked", false, "require an unchanged lockfile")
	return x
}

func (c *config) packagePackCmd() *cobra.Command {
	var output string
	var skipTests, offline, locked bool
	x := &cobra.Command{Use: "pack [path]", Short: "Build a deterministic .mosaicpkg archive", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		pkg, err := c.validatePackage(cmd.Context(), path, !skipTests)
		if err != nil {
			return err
		}
		artifact, ds := packagearchive.Pack(cmd.Context(), path, packagearchive.PackOptions{})
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("package packing failed")}
		}
		if output == "" {
			output = filepath.Join(path, "dist", strings.ReplaceAll(pkg.Manifest.Name, "/", "-")+"-"+pkg.Manifest.Version+".mosaicpkg")
		}
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return exitError{3, err}
		}
		if err := os.WriteFile(output, artifact.Bytes, 0o644); err != nil {
			return exitError{3, err}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(artifact, "", "  ")
			writeln(c.s.out, string(b))
		} else if !c.quiet {
			writef(c.s.out, "Packed package: %s@%s\nFiles: %d\nContent digest: %s\nArchive digest: %s\nOutput: %s\n", pkg.Manifest.Name, pkg.Manifest.Version, len(artifact.Files), artifact.ContentDigest, artifact.ArchiveDigest, output)
		}
		_ = offline
		_ = locked
		return nil
	}}
	x.Flags().StringVar(&output, "output", "", "archive output path")
	x.Flags().BoolVar(&skipTests, "skip-tests", false, "skip package tests")
	x.Flags().BoolVar(&offline, "offline", false, "do not access registries")
	x.Flags().BoolVar(&locked, "locked", false, "require an unchanged lockfile")
	return x
}

func (c *config) packagePublishCmd() *cobra.Command {
	var reference, registry string
	var tags []string
	var allowMove bool
	x := &cobra.Command{Use: "publish [path]", Short: "Publish a verified package as an OCI artifact", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		pkg, err := c.validatePackage(cmd.Context(), path, true)
		if err != nil {
			return err
		}
		for _, dep := range pkg.Manifest.Dependencies {
			if dep.Path != "" {
				return exitError{1, errors.New("publishing packages with local path dependencies is not supported")}
			}
		}
		artifact, ds := packagearchive.Pack(cmd.Context(), path, packagearchive.PackOptions{})
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("package packing failed")}
		}
		target := reference
		if target == "" {
			target = registry
			if target == "" {
				return exitError{2, errors.New("--reference or --registry is required")}
			}
			target = strings.TrimSuffix(target, "/") + ":" + pkg.Manifest.Version
		}
		client, err := c.ociClient(false)
		if err != nil {
			return exitError{3, err}
		}
		published, pds := client.Publish(cmd.Context(), target, artifact, baseregistry.PublishOptions{Tags: tags, AllowTagMove: allowMove})
		if pds.HasErrors() {
			c.printDiagnostics(pds)
			return exitError{1, errors.New("package publish failed")}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(published, "", "  ")
			writeln(c.s.out, string(b))
		} else {
			writef(c.s.out, "Published: %s@%s\nReference: %s\nVersion tag: %s\nContent digest: %s\n", pkg.Manifest.Name, pkg.Manifest.Version, published.Reference, published.VersionTag, published.ContentDigest)
		}
		return nil
	}}
	x.Flags().StringVar(&reference, "reference", "", "exact OCI destination")
	x.Flags().StringVar(&registry, "registry", "", "OCI repository destination")
	x.Flags().StringSliceVar(&tags, "tag", nil, "additional explicit tag")
	x.Flags().BoolVar(&allowMove, "allow-tag-move", false, "allow moving additional mutable tags")
	return x
}

func (c *config) packagePullCmd() *cobra.Command {
	var output string
	var cacheOnly bool
	x := &cobra.Command{Use: "pull <reference>", Short: "Pull and verify an OCI package", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if cacheOnly && output != "" {
			return exitError{2, errors.New("--cache-only and --output cannot be combined")}
		}
		client, err := c.ociClient(false)
		if err != nil {
			return exitError{3, err}
		}
		artifact, ds := client.Pull(cmd.Context(), args[0])
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("package pull failed")}
		}
		cache, err := c.cache()
		if err != nil {
			return exitError{3, err}
		}
		ref, _ := oci.ParseReference(args[0])
		source := "oci://" + ref.RepositoryReference()
		locked := lockfile.Package{Identity: artifact.Manifest.Name, Version: artifact.Manifest.Version, Source: source, ManifestDigest: artifact.ManifestDigest, ContentDigest: artifact.ContentDigest, ArchiveDigest: artifact.ArchiveDigest, OCIManifestDigest: artifact.OCIManifestDigest}
		if _, cds := cache.Put(cmd.Context(), artifact.Bytes, locked); cds.HasErrors() {
			c.printDiagnostics(cds)
			return exitError{1, errors.New("cache write failed")}
		}
		if output != "" {
			if err := os.WriteFile(output, artifact.Bytes, 0o644); err != nil {
				return exitError{3, err}
			}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(artifact, "", "  ")
			writeln(c.s.out, string(b))
		} else {
			writef(c.s.out, "Pulled %s@%s\nImmutable digest: %s\nCache: %s\n", artifact.Manifest.Name, artifact.Manifest.Version, artifact.OCIManifestDigest, cache.Root())
		}
		return nil
	}}
	x.Flags().StringVar(&output, "output", "", "copy archive to path")
	x.Flags().BoolVar(&cacheOnly, "cache-only", false, "only populate the cache")
	return x
}

func (c *config) inspectPackage(ctx context.Context, target string) (*packagearchive.Artifact, *mosaicpackage.Package, error) {
	info, err := os.Stat(target)
	if err == nil && info.IsDir() {
		pkg, e := c.validatePackage(ctx, target, false)
		return nil, pkg, e
	}
	if err == nil {
		data, e := os.ReadFile(target)
		if e != nil {
			return nil, nil, e
		}
		artifact, ds := packagearchive.Verify(ctx, data, packagearchive.VerifyOptions{})
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return nil, nil, exitError{1, errors.New("package verification failed")}
		}
		return artifact, nil, nil
	}
	if strings.HasPrefix(target, "oci://") {
		client, e := c.ociClient(false)
		if e != nil {
			return nil, nil, e
		}
		artifact, ds := client.Pull(ctx, target)
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return nil, nil, exitError{1, errors.New("package pull failed")}
		}
		return artifact, nil, nil
	}
	return nil, nil, err
}
func (c *config) packageInspectCmd() *cobra.Command {
	return &cobra.Command{Use: "inspect <reference-or-path>", Short: "Inspect package metadata without executing code", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		artifact, pkg, err := c.inspectPackage(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		var value any = artifact
		if pkg != nil {
			value = pkg.Manifest
		}
		b, _ := json.MarshalIndent(value, "", "  ")
		writeln(c.s.out, string(b))
		return nil
	}}
}
func (c *config) packageVerifyCmd() *cobra.Command {
	return &cobra.Command{Use: "verify <path-or-reference>", Short: "Verify package structure and integrity", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		artifact, pkg, err := c.inspectPackage(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		name, version := "", ""
		if artifact != nil {
			name, version = artifact.Manifest.Name, artifact.Manifest.Version
		} else {
			name, version = pkg.Manifest.Name, pkg.Manifest.Version
		}
		if !c.quiet {
			writef(c.s.out, "Verified package: %s@%s\nPackage structure: valid\n", name, version)
		}
		return nil
	}}
}
