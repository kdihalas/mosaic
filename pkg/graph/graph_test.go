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
