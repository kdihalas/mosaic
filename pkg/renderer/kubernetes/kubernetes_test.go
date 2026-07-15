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

func TestCustomResourceRendering(t *testing.T) {
	g := graph.New()
	fields := value.Object(map[string]value.Value{"apiVersion": value.String("monitoring.coreos.com/v1"), "kind": value.String("ServiceMonitor"), "name": value.String("catalog-metrics"), "spec": value.Object(map[string]value.Value{"endpoints": value.List([]value.Value{value.Object(map[string]value.Value{"port": value.String("metrics"), "interval": value.String("30s")})})})})
	if err := g.Add(graph.Resource{ID: "application.catalog.resource.monitor", Type: "kubernetes.CustomResource", Name: "catalog-metrics", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	a, ds := kubernetes.New().Render(context.Background(), renderer.RenderInput{Environment: "prod", Graph: g, Options: value.Object(nil)})
	if ds.HasErrors() {
		t.Fatalf("%#v", ds)
	}
	y := string(a.Files["kubernetes.yaml"])
	for _, want := range []string{"apiVersion: monitoring.coreos.com/v1", "kind: ServiceMonitor", "interval: 30s"} {
		if !strings.Contains(y, want) {
			t.Fatalf("missing %q in:\n%s", want, y)
		}
	}
}
