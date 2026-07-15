package compiler

import (
	"context"
	"testing"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/policy"
	"github.com/kdihalas/mosaic/pkg/project"
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

func hasDiagnostic(ds diagnostics.List, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}
