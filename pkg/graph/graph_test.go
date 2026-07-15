package graph_test

import (
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/value"
	"testing"
)

func TestGraphOrderingMutationAndSnapshot(t *testing.T) {
	g := graph.New()
	for _, id := range []string{"b", "a"} {
		if e := g.Add(graph.Resource{ID: graph.ResourceID(id), Type: "core.Config", Name: id, Fields: value.Object(map[string]value.Value{})}); e != nil {
			t.Fatal(e)
		}
	}
	if g.List()[0].ID != "a" {
		t.Fatal("unstable order")
	}
	if e := g.SetField("a", graph.FieldPath{"data", "x"}, value.String("y")); e != nil {
		t.Fatal(e)
	}
	if v, ok := g.ReadField("a", graph.FieldPath{"data", "x"}); !ok {
		t.Fatal("missing field")
	} else if s, _ := v.StringValue(); s != "y" {
		t.Fatal(s)
	}
	if e := g.Snapshot().SetField("a", nil, value.Null()); e == nil {
		t.Fatal("snapshot mutable")
	}
	b, _ := g.CanonicalJSON()
	x, e := graph.DecodeCanonical(b)
	if e != nil || len(x.List()) != 2 {
		t.Fatal(e)
	}
}

func TestFieldMutationsSynchronizeLabelsAndAnnotations(t *testing.T) {
	g := graph.New()
	fields := value.Object(map[string]value.Value{
		"labels":      value.Object(map[string]value.Value{"example.com/tier": value.String("base")}),
		"annotations": value.Object(map[string]value.Value{"example.com/owner": value.String("base")}),
	})
	if err := g.Add(graph.Resource{
		ID: "app", Type: "core.Config", Name: "app", Fields: fields,
		Labels:      map[string]string{"example.com/tier": "base"},
		Annotations: map[string]string{"example.com/owner": "base"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetField("app", graph.FieldPath{"labels", "example.com/tier"}, value.String("local")); err != nil {
		t.Fatal(err)
	}
	if err := g.SetField("app", graph.FieldPath{"annotations", "example.com/owner"}, value.String("local")); err != nil {
		t.Fatal(err)
	}
	r, _ := g.Get("app")
	if r.Labels["example.com/tier"] != "local" || r.Annotations["example.com/owner"] != "local" {
		t.Fatalf("metadata mirrors were not updated: labels=%v annotations=%v", r.Labels, r.Annotations)
	}
	if err := g.DeleteField("app", graph.FieldPath{"labels", "example.com/tier"}); err != nil {
		t.Fatal(err)
	}
	if err := g.DeleteField("app", graph.FieldPath{"annotations"}); err != nil {
		t.Fatal(err)
	}
	r, _ = g.Get("app")
	if len(r.Labels) != 0 || r.Annotations != nil {
		t.Fatalf("metadata mirrors were not deleted: labels=%v annotations=%v", r.Labels, r.Annotations)
	}
}
