package mosaic_test

import (
	"context"
	"testing"

	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/renderer/kubernetes"
)

func TestCatalogEnvironments(t *testing.T) {
	ctx := context.Background()
	p, ds := project.Load(ctx, "examples/catalog-platform", project.LoadOptions{})
	if ds.HasErrors() {
		t.Fatalf("load: %#v", ds)
	}
	c := compiler.New(compiler.NewOptions{})
	want := map[string]int{"dev": 4, "stage": 5, "prod": 6}
	for env, n := range want {
		r, ds := c.Compile(ctx, p, compiler.Options{Environment: env})
		if ds.HasErrors() {
			t.Fatalf("%s: %#v", env, ds)
		}
		if got := len(r.Graph.List()); got != n {
			t.Fatalf("%s resources=%d want %d", env, got, n)
		}
		a, ds := kubernetes.New().Render(ctx, renderer.RenderInput{Environment: env, Graph: r.Graph, Provenance: r.Provenance, Options: r.Metadata.TargetOptions})
		if ds.HasErrors() {
			t.Fatalf("render %s: %#v", env, ds)
		}
		if len(a.Files["kubernetes.yaml"]) == 0 {
			t.Fatal("empty YAML")
		}
	}
}
