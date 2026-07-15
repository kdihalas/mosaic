package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInitValidateBuildAndVersion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "catalog")
	run := func(args ...string) (string, string, int) {
		var out, err bytes.Buffer
		code := Execute(context.Background(), args, bytes.NewReader(nil), &out, &err)
		return out.String(), err.String(), code
	}
	if _, e, c := run("init", dir); c != 0 {
		t.Fatalf("init %d %s", c, e)
	}
	if _, e, c := run("--project", dir, "fmt", "--check"); c != 0 {
		t.Fatalf("fmt %d %s", c, e)
	}
	if _, e, c := run("--project", dir, "validate", "prod"); c != 0 {
		t.Fatalf("validate %d %s", c, e)
	}
	out := filepath.Join(dir, "dist", "prod")
	if _, e, c := run("--project", dir, "build", "prod", "--output", out); c != 0 {
		t.Fatalf("build %d %s", c, e)
	}
	if _, e := os.Stat(filepath.Join(out, "bundle.json")); e != nil {
		t.Fatal(e)
	}
	if o, e, c := run("version"); c != 0 || e != "" || o == "" {
		t.Fatalf("version %d %s", c, e)
	}
}

func TestEverySubcommandHasDescription(t *testing.T) {
	c := &config{s: streams{in: bytes.NewReader(nil), out: &bytes.Buffer{}, err: &bytes.Buffer{}}}
	want := map[string]bool{"init": true, "fmt": true, "parse": true, "validate": true, "build": true, "inspect": true, "explain": true, "diff": true, "test": true, "version": true, "lex": true}
	for _, cmd := range c.root().Commands() {
		if !want[cmd.Name()] {
			continue
		}
		if cmd.Short == "" {
			t.Errorf("command %q has no description", cmd.Name())
		}
		delete(want, cmd.Name())
	}
	for name := range want {
		t.Errorf("command %q is not registered", name)
	}
}
