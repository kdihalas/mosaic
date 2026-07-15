package kubernetes_test

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/renderer/kubernetes"
	"github.com/kdihalas/mosaic/pkg/value"
)

func TestTypedNumbersAreNotQuotedInYAML(t *testing.T) {
	g := graph.New()
	fields := value.Object(map[string]value.Value{
		"image":    value.String("example.com/app:v1"),
		"replicas": value.Int(big.NewInt(3)),
		"ports":    value.List([]value.Value{value.Object(map[string]value.Value{"name": value.String("http"), "containerPort": value.Int(big.NewInt(8080))})}),
	})
	if err := g.Add(graph.Resource{ID: "application.app.workload.main", Type: "core.Workload", Name: "app", Fields: fields, Labels: map[string]string{"app.kubernetes.io/name": "app"}}); err != nil {
		t.Fatal(err)
	}
	a, ds := kubernetes.New().Render(context.Background(), renderer.RenderInput{Environment: "test", Graph: g, Options: value.Object(map[string]value.Value{})})
	if ds.HasErrors() {
		t.Fatalf("%#v", ds)
	}
	y := string(a.Files["kubernetes.yaml"])
	for _, want := range []string{"replicas: 3", "containerPort: 8080"} {
		if !strings.Contains(y, want) {
			t.Fatalf("missing %q in:\n%s", want, y)
		}
	}
	for _, bad := range []string{`replicas: "3"`, `containerPort: "8080"`} {
		if strings.Contains(y, bad) {
			t.Fatalf("numeric value was quoted: %s", bad)
		}
	}
	if strings.Contains(string(a.Files["kubernetes.json"]), `"replicas": "3"`) {
		t.Fatal("JSON replica count was quoted")
	}
}
