package compiler

import (
	"context"
	"strings"
	"testing"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/policy"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/renderer/kubernetes"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

func selectionProject(t *testing.T) *project.Project {
	t.Helper()
	return project.New(".", "selection", []source.File{source.NewFile("selection.mosaic", []byte(`
module App {
    workload "main" { name = "app" image = "example.com/app:v1" replicas = 1 }
    extension workload.main.replicas
}
use App as app {}
variant scale { set app.workload.main.replicas = 3 }
variant rescale { set app.workload.main.replicas = 5 }
policy minimumReplicas {
    downgradeAllowed = true
    select Workload { deny replicas < 2 { message = "too small" } }
}
policy imagePolicy {
    select Workload { deny image.endsWith(":latest") { message = "latest forbidden" } }
}
environment prod { use app target kubernetes { namespace = "prod" } }
`))})
}

func TestCompileInputAppliesSelectedVariantsInOrder(t *testing.T) {
	result, ds := New(NewOptions{}).CompileInput(context.Background(), Input{
		RootProject: selectionProject(t), Environment: "prod", Variants: []string{"scale"},
	})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	v, ok := result.Graph.ReadField(graph.ResourceID("application.app.workload.main"), graph.FieldPath{"replicas"})
	if !ok {
		t.Fatal("missing replicas")
	}
	i, _ := v.IntValue()
	if got := i.Int64(); got != 3 {
		t.Fatalf("replicas = %d, want 3", got)
	}
}

func TestCompileInputRejectsDuplicateSelectedVariant(t *testing.T) {
	_, ds := New(NewOptions{}).CompileInput(context.Background(), Input{
		RootProject: selectionProject(t), Environment: "prod", Variants: []string{"scale", "scale"},
	})
	if !ds.HasErrors() || !hasDiagnostic(ds, "SEM007") {
		t.Fatalf("expected SEM007, got %v", ds)
	}
}

func TestPolicySelectionAndAllowedDowngrade(t *testing.T) {
	result, ds := New(NewOptions{}).CompileInput(context.Background(), Input{
		RootProject: selectionProject(t), Environment: "prod",
		Policy: policy.Options{Include: []string{"minimumReplicas"}, FailureMode: policy.FailureModeWarn},
	})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	if len(result.PolicyReport.Results) != 1 {
		t.Fatalf("results = %#v", result.PolicyReport.Results)
	}
	got := result.PolicyReport.Results[0]
	if got.Severity != diagnostics.SeverityWarning || !got.DowngradeAllowed {
		t.Fatalf("result = %#v", got)
	}
}

func TestResolveOverlappingMergeAtQuotedKey(t *testing.T) {
	p := project.New(".", "label-resolution", []source.File{source.NewFile("labels.mosaic", []byte(`
module App {
    workload "main" {
        name = "app"
        image = "example.com/app:v1"
        labels { "app.example.com/tier" = "base" }
    }
    extension workload.main.labels
}
use App as app {}
variant first {
    merge app.workload.main.labels { "app.example.com/tier" = "first" }
}
variant second {
    merge app.workload.main.labels { "app.example.com/tier" = "second" }
}
environment prod {
    use app
    apply first
    apply second
    resolve app.workload.main.labels["app.example.com/tier"] = "local"
    target kubernetes { namespace = "prod" }
}
`))})
	result, ds := New(NewOptions{}).CompileInput(context.Background(), Input{
		RootProject: p, Environment: "prod",
	})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	if len(result.Conflicts) != 1 || !result.Conflicts[0].Resolved {
		t.Fatalf("conflicts = %#v, want one resolved conflict", result.Conflicts)
	}
	v, ok := result.Graph.ReadField(
		graph.ResourceID("application.app.workload.main"),
		graph.FieldPath{"labels", "app.example.com/tier"},
	)
	if !ok {
		t.Fatal("missing resolved label")
	}
	got, _ := v.StringValue()
	if got != "local" {
		t.Fatalf("label = %q, want local", got)
	}
	rendered, rds := kubernetes.New().Render(context.Background(), renderer.RenderInput{
		Environment: result.Environment,
		Graph:       result.Graph,
		Provenance:  result.Provenance,
		Options:     result.Metadata.TargetOptions,
	})
	if rds.HasErrors() {
		t.Fatal(rds)
	}
	if yaml := string(rendered.Files["kubernetes.yaml"]); !strings.Contains(yaml, "app.example.com/tier: local") {
		t.Fatalf("rendered YAML does not contain resolved label:\n%s", yaml)
	}
}

func hasDiagnostic(ds diagnostics.List, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}
