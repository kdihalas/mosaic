// Package cli implements the mosaic command as a thin adapter over public APIs.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kdihalas/mosaic/pkg/bundle"
	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	mdiff "github.com/kdihalas/mosaic/pkg/diff"
	"github.com/kdihalas/mosaic/pkg/explain"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/renderer/kubernetes"
	"github.com/kdihalas/mosaic/pkg/syntax/formatter"
	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/parser"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
	mtesting "github.com/kdihalas/mosaic/pkg/testing"
	"github.com/spf13/cobra"
)

var Version = "0.1.0"
var LanguageVersion = "v1alpha1"

type streams struct {
	in       io.Reader
	out, err io.Writer
}
type config struct {
	project, format, cacheDir string
	plainHTTP                 bool
	noColor, verbose, quiet   bool
	max                       int
	s                         streams
}
type exitError struct {
	code int
	err  error
}

func writef(w io.Writer, format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }
func writeln(w io.Writer, args ...any)               { _, _ = fmt.Fprintln(w, args...) }
func writeText(w io.Writer, s string)                { _, _ = io.WriteString(w, s) }
func writeBytes(w io.Writer, b []byte)               { _, _ = w.Write(b) }

func (e exitError) Error() string { return e.err.Error() }
func Execute(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	c := &config{s: streams{in, out, errOut}, format: "text", max: 100}
	root := c.root()
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		var x exitError
		if errors.As(err, &x) {
			if x.err != nil {
				writeln(errOut, x.err)
			}
			return x.code
		}
		writeln(errOut, err)
		return 2
	}
	return 0
}
func (c *config) root() *cobra.Command {
	r := &cobra.Command{Use: "mosaic", Short: "Typed configuration compiler", SilenceUsage: true, SilenceErrors: true}
	r.SetIn(c.s.in)
	r.SetOut(c.s.out)
	r.SetErr(c.s.err)
	f := r.PersistentFlags()
	f.StringVar(&c.project, "project", ".", "project root")
	f.StringVar(&c.format, "format", "text", "output format")
	f.BoolVar(&c.noColor, "no-color", false, "disable color")
	f.IntVar(&c.max, "max-diagnostics", 100, "maximum diagnostics")
	f.BoolVar(&c.verbose, "verbose", false, "verbose output")
	f.BoolVar(&c.quiet, "quiet", false, "suppress summaries")
	f.StringVar(&c.cacheDir, "cache-dir", "", "package cache directory")
	f.BoolVar(&c.plainHTTP, "plain-http", false, "allow plaintext HTTP for OCI registries")
	r.AddCommand(c.initCmd(), c.fmtCmd(), c.parseCmd(), c.validateCmd(), c.buildCmd(), c.inspectCmd(), c.explainCmd(), c.diffCmd(), c.testCmd(), c.versionCmd(), c.lexCmd(), c.packageCmd(), c.depsCmd(), c.cacheCmd())
	return r
}
func (c *config) load(ctx context.Context) (*project.Project, error) {
	p, ds := project.Load(ctx, c.project, project.LoadOptions{})
	if ds.HasErrors() {
		c.printDiagnostics(ds)
		return nil, exitError{3, errors.New("project loading failed")}
	}
	return p, nil
}
func (c *config) compile(ctx context.Context, env string) (*compiler.Result, error) {
	return c.compileWithDependencies(ctx, env, dependencyRunOptions{locked: true})
}
func (c *config) compileWithDependencies(ctx context.Context, env string, options dependencyRunOptions) (*compiler.Result, error) {
	p, packages, e := c.loadCompilation(ctx, options)
	if e != nil {
		return nil, e
	}
	r, ds := compiler.New(compiler.NewOptions{}).CompileInput(ctx, compiler.Input{RootProject: p, Packages: packages, Environment: env})
	if len(ds) > 0 {
		c.printDiagnostics(ds)
	}
	if ds.HasErrors() {
		return nil, exitError{1, errors.New("compilation failed")}
	}
	return r, nil
}
func (c *config) printDiagnostics(ds diagnostics.List) {
	if c.format == "json" {
		b, _ := json.MarshalIndent(ds, "", "  ")
		writeln(c.s.err, string(b))
		return
	}
	for _, d := range ds {
		writef(c.s.err, "%s[%s]: %s\n", d.Severity, d.Code, d.Message)
		if d.Span.SourceName != "" {
			writef(c.s.err, "  %s:%d:%d\n", d.Span.SourceName, d.Span.Start.Line, d.Span.Start.Column)
		}
		for _, note := range d.Notes {
			writef(c.s.err, "  %s\n", note)
		}
		for _, related := range d.Related {
			writef(c.s.err, "  %s: %s:%d:%d\n", related.Message, related.Span.SourceName, related.Span.Start.Line, related.Span.Start.Column)
		}
		if d.Suggestion != "" {
			writef(c.s.err, "  suggestion: %s\n", d.Suggestion)
		}
	}
}
func (c *config) initCmd() *cobra.Command {
	return &cobra.Command{Use: "init [directory]", Short: "Create a complete Mosaic project", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		for n, data := range scaffold {
			path := filepath.Join(dir, filepath.FromSlash(n))
			if _, e := os.Stat(path); e == nil {
				return exitError{3, fmt.Errorf("refusing to overwrite %s", path)}
			}
			if e := os.MkdirAll(filepath.Dir(path), 0o755); e != nil {
				return exitError{3, e}
			}
			content := []byte(data)
			if filepath.Ext(n) == ".mosaic" {
				src := source.NewFile(n, content)
				lr := lexer.Lex(src, lexer.Options{})
				pr := parser.Parse(src, lr.Tokens, parser.Options{})
				if !lr.Diagnostics.HasErrors() && !pr.Diagnostics.HasErrors() {
					content = append(formatter.Format(pr.File), '\n')
				}
			}
			if e := os.WriteFile(path, content, 0o644); e != nil {
				return exitError{3, e}
			}
		}
		if !c.quiet {
			writef(c.s.out, "Initialised Mosaic project in %s\n", dir)
		}
		return nil
	}}
}
func (c *config) fmtCmd() *cobra.Command {
	var check, stdout bool
	x := &cobra.Command{Use: "fmt [paths...]", Short: "Format Mosaic source files", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, e := c.load(cmd.Context())
		if e != nil {
			return e
		}
		files := p.Files
		if len(args) > 0 {
			files = nil
			for _, a := range args {
				st, er := os.Stat(a)
				if er != nil {
					return exitError{3, er}
				}
				if st.IsDir() {
					er = filepath.WalkDir(a, func(path string, d os.DirEntry, er error) error {
						if er == nil && !d.IsDir() && filepath.Ext(path) == ".mosaic" {
							b, x := os.ReadFile(path)
							if x != nil {
								return x
							}
							files = append(files, source.NewFile(path, b))
						}
						return er
					})
					if er != nil {
						return exitError{3, er}
					}
				} else {
					b, er := os.ReadFile(a)
					if er != nil {
						return exitError{3, er}
					}
					files = append(files, source.NewFile(a, b))
				}
			}
		}
		changed := false
		for _, f := range files {
			lr := lexer.Lex(f, lexer.Options{MaxDiagnostics: c.max})
			pr := parser.Parse(f, lr.Tokens, parser.Options{MaxDiagnostics: c.max})
			ds := lr.Diagnostics.Append(pr.Diagnostics)
			if ds.HasErrors() {
				c.printDiagnostics(ds)
				return exitError{1, errors.New("formatting refused due to syntax errors")}
			}
			b := append(formatter.Format(pr.File), '\n')
			if !bytes.Equal(b, f.Content) {
				changed = true
				if stdout {
					writeBytes(c.s.out, b)
				} else if !check {
					path := f.Name
					if !filepath.IsAbs(path) {
						path = filepath.Join(p.Root, filepath.FromSlash(path))
					}
					if er := os.WriteFile(path, b, 0o644); er != nil {
						return exitError{3, er}
					}
				}
			}
		}
		if check && changed {
			return exitError{1, errors.New("files require formatting")}
		}
		return nil
	}}
	x.Flags().BoolVar(&check, "check", false, "check formatting")
	x.Flags().BoolVar(&stdout, "stdout", false, "write one formatted file to stdout")
	return x
}
func (c *config) parseCmd() *cobra.Command {
	var trivia bool
	x := &cobra.Command{Use: "parse [paths...]", Short: "Parse Mosaic source and print its syntax tree", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, e := c.load(cmd.Context())
		if e != nil {
			return e
		}
		files := p.Files
		if len(args) > 0 {
			files = nil
			for _, a := range args {
				b, er := os.ReadFile(a)
				if er != nil {
					return exitError{3, er}
				}
				files = append(files, source.NewFile(a, b))
			}
		}
		var failed bool
		for _, f := range files {
			lr := lexer.Lex(f, lexer.Options{MaxDiagnostics: c.max})
			pr := parser.Parse(f, lr.Tokens, parser.Options{MaxDiagnostics: c.max})
			ds := lr.Diagnostics.Append(pr.Diagnostics)
			if len(ds) > 0 {
				c.printDiagnostics(ds)
			}
			failed = failed || ds.HasErrors()
			if c.format == "json" {
				b, _ := json.MarshalIndent(pr.File, "", "  ")
				writeln(c.s.out, string(b))
			} else {
				writeText(c.s.out, parser.Tree(pr.File))
			}
			_ = trivia
		}
		if failed {
			return exitError{1, errors.New("parse failed")}
		}
		return nil
	}}
	x.Flags().BoolVar(&trivia, "include-trivia", false, "include trivia in JSON")
	return x
}
func (c *config) validateCmd() *cobra.Command {
	var locked, offline, vendorMode, updateLock, noReplacements bool
	x := &cobra.Command{Use: "validate [environment]", Short: "Validate a project or environment without rendering", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, packages, e := c.loadCompilation(cmd.Context(), dependencyRunOptions{locked: locked, offline: offline, vendor: vendorMode, updateLock: updateLock})
		if e != nil {
			return e
		}
		if noReplacements && len(p.Config.Replace) > 0 {
			return exitError{1, errors.New("project replacements are not permitted")}
		}
		co := compiler.New(compiler.NewOptions{})
		if len(args) == 0 {
			a, ds := co.AnalyzeInput(cmd.Context(), compiler.Input{RootProject: p, Packages: packages})
			if len(ds) > 0 {
				c.printDiagnostics(ds)
			}
			if ds.HasErrors() {
				return exitError{1, errors.New("validation failed")}
			}
			if !c.quiet {
				writef(c.s.out, "Validated %d source files.\nNo errors.\n", len(a.Files))
			}
			return nil
		}
		r, ds := co.CompileInput(cmd.Context(), compiler.Input{RootProject: p, Packages: packages, Environment: args[0]})
		if len(ds) > 0 {
			c.printDiagnostics(ds)
		}
		if ds.HasErrors() {
			return exitError{1, errors.New("validation failed")}
		}
		if !c.quiet {
			writef(c.s.out, "Validated environment %s (%d resources).\nNo errors.\n", r.Environment, len(r.Graph.List()))
		}
		return nil
	}}
	x.Flags().BoolVar(&locked, "locked", false, "do not modify mosaic.lock")
	x.Flags().BoolVar(&offline, "offline", false, "use local sources and verified cache only")
	x.Flags().BoolVar(&vendorMode, "vendor", false, "use vendor/mosaic only")
	x.Flags().BoolVar(&updateLock, "update-lock", false, "resolve and update mosaic.lock")
	x.Flags().BoolVar(&noReplacements, "no-replacements", false, "reject project replacements")
	return x
}
func (c *config) buildCmd() *cobra.Command {
	var output string
	var clean, locked, offline, vendorMode, updateLock bool
	x := &cobra.Command{Use: "build <environment>", Short: "Compile an environment into a deterministic bundle", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		r, e := c.compileWithDependencies(cmd.Context(), args[0], dependencyRunOptions{locked: locked, offline: offline, vendor: vendorMode, updateLock: updateLock})
		if e != nil {
			return e
		}
		a, ds := kubernetes.New().Render(cmd.Context(), renderer.RenderInput{Environment: r.Environment, Graph: r.Graph, Provenance: r.Provenance, Options: r.Metadata.TargetOptions})
		if len(ds) > 0 {
			c.printDiagnostics(ds)
		}
		if ds.HasErrors() {
			return exitError{1, errors.New("render failed")}
		}
		if c.format == "yaml" || c.format == "json" {
			name := "kubernetes." + c.format
			if output == "" {
				writeBytes(c.s.out, a.Files[name])
				return nil
			}
			return os.WriteFile(output, a.Files[name], 0o644)
		}
		if output == "" {
			output = filepath.Join(c.project, "dist", r.Environment)
		}
		b, er := bundle.Build(r, *a, bundle.BuildOptions{})
		if er != nil {
			return exitError{1, er}
		}
		_ = clean // WriteDirectory safely replaces an existing bundle.
		if er = bundle.WriteDirectory(cmd.Context(), b, output); er != nil {
			return exitError{3, er}
		}
		if !c.quiet {
			writef(c.s.out, "Built environment: %s\nOutput: %s\nGraph digest: %s\nBundle digest: %s\n", r.Environment, output, b.Manifest.GraphDigest, b.Manifest.BundleDigest)
		}
		return nil
	}}
	x.Flags().StringVar(&output, "output", "", "output path")
	x.Flags().BoolVar(&clean, "clean", false, "replace existing output")
	x.Flags().BoolVar(&locked, "locked", false, "do not modify mosaic.lock")
	x.Flags().BoolVar(&offline, "offline", false, "use local sources and verified cache only")
	x.Flags().BoolVar(&vendorMode, "vendor", false, "use vendor/mosaic only")
	x.Flags().BoolVar(&updateLock, "update-lock", false, "resolve and update mosaic.lock")
	return x
}
func (c *config) inspectCmd() *cobra.Command {
	var resourceID, phase string
	x := &cobra.Command{Use: "inspect <environment>", Short: "Inspect graph state at a compiler phase", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		r, e := c.compile(cmd.Context(), args[0])
		if e != nil {
			return e
		}
		g := r.Graph
		if phase != "" {
			g = r.Snapshots[compiler.PhaseName(phase)]
			if g == nil {
				return exitError{1, fmt.Errorf("unknown phase %s", phase)}
			}
		}
		if c.format == "json" {
			b, _ := g.CanonicalJSON()
			writeln(c.s.out, string(b))
			return nil
		}
		for _, res := range g.List() {
			if resourceID != "" && string(res.ID) != resourceID {
				continue
			}
			b, _ := res.Fields.CanonicalJSON()
			writef(c.s.out, "%s\n  type: %s\n  name: %s\n  fields: %s\n", res.ID, res.Type, res.Name, b)
		}
		return nil
	}}
	x.Flags().StringVar(&resourceID, "resource", "", "resource ID")
	x.Flags().StringVar(&phase, "phase", "", "compiler phase")
	return x
}
func (c *config) explainCmd() *cobra.Command {
	return &cobra.Command{Use: "explain <environment> <resource-id> [field-path]", Short: "Explain a resource or field using provenance", Args: cobra.RangeArgs(2, 3), RunE: func(cmd *cobra.Command, args []string) error {
		r, e := c.compile(cmd.Context(), args[0])
		if e != nil {
			return e
		}
		var p graph.FieldPath
		if len(args) == 3 {
			p, e = graph.ParseFieldPath(args[2])
			if e != nil {
				return exitError{2, e}
			}
		}
		x, e := explain.Query(r, graph.ResourceID(args[1]), p)
		if e != nil {
			return exitError{1, e}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(x, "", "  ")
			writeln(c.s.out, string(b))
			return nil
		}
		v, _ := x.Value.CanonicalJSON()
		writef(c.s.out, "Resource:\n  %s\n\nField:\n  %s\n\nFinal value:\n  %s\n\nHistory:\n", x.Resource.ID, p.String(), v)
		for _, h := range x.History {
			writef(c.s.out, "%d. %s by %s %s\n", h.Sequence, h.Action, h.Owner.Kind, h.Owner.Name)
		}
		return nil
	}}
}
func (c *config) diffCmd() *cobra.Command {
	var fail bool
	x := &cobra.Command{Use: "diff <old-bundle> <new-bundle>", Short: "Compare two bundles semantically", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		a, ds := bundle.ReadDirectory(cmd.Context(), args[0])
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{3, errors.New("cannot read old bundle")}
		}
		b, ds := bundle.ReadDirectory(cmd.Context(), args[1])
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{3, errors.New("cannot read new bundle")}
		}
		r := mdiff.Compare(a.Manifest.Environment, b.Manifest.Environment, a.Graph, b.Graph)
		if c.format == "json" {
			writeBytes(c.s.out, r.JSON())
		} else {
			writeText(c.s.out, r.Text())
		}
		if fail && len(r.Changes) > 0 {
			return exitError{4, errors.New("differences found")}
		}
		return nil
	}}
	x.Flags().BoolVar(&fail, "fail-on-change", false, "exit 4 when changes exist")
	return x
}
func (c *config) testCmd() *cobra.Command {
	var locked, offline, vendorMode, updateLock bool
	x := &cobra.Command{Use: "test [paths...]", Short: "Run Mosaic configuration tests", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, packages, e := c.loadCompilation(cmd.Context(), dependencyRunOptions{locked: locked, offline: offline, vendor: vendorMode, updateLock: updateLock})
		if e != nil {
			return e
		}
		r, ds := mtesting.RunInput(cmd.Context(), compiler.Input{RootProject: p, Packages: packages}, compiler.New(compiler.NewOptions{}))
		if ds.HasErrors() {
			c.printDiagnostics(ds)
			return exitError{1, errors.New("test analysis failed")}
		}
		if c.format == "json" {
			b, _ := json.MarshalIndent(r, "", "  ")
			writeln(c.s.out, string(b))
		} else {
			for _, x := range r.Cases {
				status := "PASS"
				if !x.Passed {
					status = "FAIL"
				}
				writef(c.s.out, "%s %s", status, x.Name)
				if x.Message != "" {
					writef(c.s.out, ": %s", x.Message)
				}
				writeln(c.s.out)
			}
			if !c.quiet {
				writef(c.s.out, "\n%d passed, %d failed\n", r.Passed, r.Failed)
			}
		}
		if r.Failed > 0 {
			return exitError{1, errors.New("configuration tests failed")}
		}
		return nil
	}}
	x.Flags().BoolVar(&locked, "locked", false, "do not modify mosaic.lock")
	x.Flags().BoolVar(&offline, "offline", false, "use local sources and verified cache only")
	x.Flags().BoolVar(&vendorMode, "vendor", false, "use vendor/mosaic only")
	x.Flags().BoolVar(&updateLock, "update-lock", false, "resolve and update mosaic.lock")
	return x
}
func (c *config) versionCmd() *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Print compiler, language, and bundle versions", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if c.format == "json" {
			writef(c.s.out, "{\"version\":%q,\"languageVersion\":%q,\"bundleFormat\":%q}\n", Version, LanguageVersion, bundle.FormatVersion)
		} else {
			writef(c.s.out, "Mosaic %s\nLanguage %s\nBundle format %s\n", Version, LanguageVersion, bundle.FormatVersion)
		}
		return nil
	}}
}
func (c *config) lexCmd() *cobra.Command {
	return &cobra.Command{Use: "lex <file>", Short: "Print tokens from the existing Mosaic lexer", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		b, e := os.ReadFile(args[0])
		if e != nil {
			return exitError{3, e}
		}
		r := lexer.Lex(source.NewFile(args[0], b), lexer.Options{MaxDiagnostics: c.max})
		if len(r.Diagnostics) > 0 {
			c.printDiagnostics(r.Diagnostics)
		}
		for _, t := range r.Tokens {
			writeln(c.s.out, token.Format(t))
		}
		if r.Diagnostics.HasErrors() {
			return exitError{1, errors.New("lex failed")}
		}
		return nil
	}}
}

var scaffold = map[string]string{
	"mosaic.toml":                        "name = \"catalog-platform\"\nlanguage_version = \"v1alpha1\"\nsources = [\"**/*.mosaic\"]\nexclude = [\"dist/**\"]\ndefault_output = \"dist\"\n",
	"modules/http-service/types.mosaic":  "type HttpServiceInput {\n    name: string\n    image: ImageReference\n    owner: string\n    replicas: int = 1\n}\n",
	"modules/http-service/module.mosaic": "module HttpService(input: HttpServiceInput) {\n    config \"runtime\" { name = input.name\n        data = input.config\n        labels { \"something.com/owner\" = input.owner }\n    }\n    serviceAccount \"main\" { name = input.name\n        labels { \"something.com/owner\" = input.owner }\n    }\n    workload \"main\" { name = input.name\n        image = input.image\n        replicas = input.replicas\n        serviceAccount = serviceAccount.main\n        labels { \"app.kubernetes.io/name\" = input.name\n            \"something.com/owner\" = input.owner\n        }\n    }\n    expose \"http\" { name = input.name\n        workload = workload.main\n    }\n}\n",
	"applications/catalog-api.mosaic":    "use HttpService as catalog {\n    name = \"catalog-api\"\n    image = \"ghcr.io/acme/catalog-api:1.4.0\"\n    owner = \"catalog-team\"\n    replicas = 1\n    config = { APP_ENV = \"base\" }\n}\n",
	"variants/dev.mosaic":                "variant development {\n    set catalog.workload.main.replicas = 1\n}\n",
	"variants/stage.mosaic":              "variant staging {\n    set catalog.workload.main.replicas = 2\n    enable autoscaling { target = catalog.workload.main\n        minimumReplicas = 2\n        maximumReplicas = 6\n    }\n}\n",
	"variants/prod.mosaic":               "variant production {\n    set catalog.workload.main.replicas = 3\n    enable autoscaling { target = catalog.workload.main\n        minimumReplicas = 3\n        maximumReplicas = 12\n    }\n    enable disruptionProtection { target = catalog.workload.main\n        minimumAvailable = 2\n    }\n}\n",
	"policies/baseline.mosaic":           "policy validContainerImages {\n    select Workload {\n        deny image.endsWith(\":latest\") { message = \"Latest tags are not permitted.\" }\n    }\n}\n",
	"environments/dev.mosaic":            "environment dev { use catalog\n    apply development\n    target kubernetes { namespace = \"catalog-dev\" }\n}\n",
	"environments/stage.mosaic":          "environment stage { use catalog\n    apply staging\n    target kubernetes { namespace = \"catalog-stage\" }\n}\n",
	"environments/prod.mosaic":           "environment prod { use catalog\n    apply production\n    target kubernetes { namespace = \"catalog-prod\" }\n}\n",
	"tests/production.mosaic":            "test productionUsesStableImage {\n    build prod\n    assert catalog.workload.main.image == \"ghcr.io/acme/catalog-api:1.4.0\"\n}\n\ntest productionHasAutoscaling {\n    build prod\n    assert catalog.capability.autoscaling != null\n}\n",
	"README.md":                          "# Catalog platform\n\nGenerated by Mosaic.\n",
}
