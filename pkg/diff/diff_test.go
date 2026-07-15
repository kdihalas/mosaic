package diff_test

import (
	"github.com/kdihalas/mosaic/pkg/diff"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/value"
	"testing"
)

func TestSemanticChanges(t *testing.T) {
	a, b := graph.New(), graph.New()
	_ = a.Add(graph.Resource{ID: "r", Type: "core.Workload", Name: "r", Fields: value.Object(map[string]value.Value{"image": value.String("old")})})
	_ = b.Add(graph.Resource{ID: "r", Type: "core.Workload", Name: "r", Fields: value.Object(map[string]value.Value{"image": value.String("new")})})
	r := diff.Compare("a", "b", a, b)
	if len(r.Changes) != 1 || r.Changes[0].Kind != diff.ScalarChanged {
		t.Fatalf("%#v", r)
	}
}
