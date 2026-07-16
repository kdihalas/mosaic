package compiler

import (
	"context"
	"testing"

	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

func conditionalProject() *project.Project {
	return project.New(".", "conditional", []source.File{source.NewFile("conditional.mosaic", []byte(`
module Service(input: Input) {
    workload "main" {
        name = input.name
        image = "example.com/app:v1"
        replicas = 1
    }
    export workload.main
    extension workload.main.replicas

    when(input.monitoring) {
        resource "monitor" {
            apiVersion = "monitoring.coreos.com/v1"
            kind = "ServiceMonitor"
            name = input.name
            spec { interval = "30s" selector = "app" }
        }
        export resource.monitor
        extension resource.monitor.spec.interval
        protected resource.monitor.spec.selector
    }
}
use Service as disabled { name = "disabled" monitoring = false }
use Service as enabled { name = "enabled" monitoring = true }
variant tuneDisabled {
    when(present(disabled.resource.monitor)) {
        set disabled.resource.monitor.spec.interval = "15s"
    }
}
variant tuneEnabled {
    when(present(enabled.resource.monitor)) {
        set enabled.resource.monitor.spec.interval = "15s"
    }
}
environment off { use disabled apply tuneDisabled target kubernetes { namespace = "off" } }
environment on { use enabled apply tuneEnabled target kubernetes { namespace = "on" } }
`))})
}

func TestConditionalResourceAndOptionalExport(t *testing.T) {
	compiler := New(NewOptions{})
	off, ds := compiler.CompileInput(context.Background(), Input{RootProject: conditionalProject(), Environment: "off"})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	if _, ok := off.Graph.Get("application.disabled.resource.monitor"); ok {
		t.Fatal("disabled optional resource was created")
	}
	if len(off.Instances) != 1 || len(off.Instances[0].Exports) != 2 || off.Instances[0].Exports[0].Present {
		t.Fatalf("disabled exports = %#v", off.Instances)
	}

	on, ds := compiler.CompileInput(context.Background(), Input{RootProject: conditionalProject(), Environment: "on"})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	value, ok := on.Graph.ReadField("application.enabled.resource.monitor", graph.FieldPath{"spec", "interval"})
	if !ok {
		t.Fatal("enabled optional resource is missing")
	}
	interval, _ := value.StringValue()
	if interval != "15s" {
		t.Fatalf("interval = %q", interval)
	}
	if !on.Instances[0].Exports[0].Optional || !on.Instances[0].Exports[0].Present {
		t.Fatalf("enabled exports = %#v", on.Instances[0].Exports)
	}
}

func TestOptionalExportRequiresPresentGuard(t *testing.T) {
	p := conditionalProject()
	p.Files = append(p.Files, source.NewFile("unguarded.mosaic", []byte(`
variant unguarded { set enabled.resource.monitor.spec.interval = "5s" }
environment invalid { use enabled apply unguarded target kubernetes { namespace = "invalid" } }
`)))
	_, ds := New(NewOptions{}).CompileInput(context.Background(), Input{RootProject: p, Environment: "invalid"})
	if !hasDiagnostic(ds, "SEM046") {
		t.Fatalf("expected SEM046, got %v", ds)
	}
}

func TestPrivateResourceCannotBeMutated(t *testing.T) {
	p := project.New(".", "private", []source.File{source.NewFile("private.mosaic", []byte(`
module App {
    workload "main" { name = "app" image = "example.com/app:v1" }
    config "private" { name = "private" data = {} }
    export workload.main
    extension config.private.data
}
use App as app {}
variant mutate { set app.config.private.data = { key = "value" } }
environment prod { use app apply mutate target kubernetes { namespace = "prod" } }
`))})
	_, ds := New(NewOptions{}).CompileInput(context.Background(), Input{RootProject: p, Environment: "prod"})
	if !hasDiagnostic(ds, "SEM049") {
		t.Fatalf("expected SEM049, got %v", ds)
	}
}

func TestConditionalMutationContractMustMatchResourceGuard(t *testing.T) {
	p := project.New(".", "contract", []source.File{source.NewFile("contract.mosaic", []byte(`
module App(input: Input) {
    workload "main" { name = "app" image = "example.com/app:v1" replicas = 1 }
    when(input.mutable) { extension workload.main.replicas }
}
use App as app { mutable = true }
environment prod { use app target kubernetes { namespace = "prod" } }
`))})
	_, ds := New(NewOptions{}).CompileInput(context.Background(), Input{RootProject: p, Environment: "prod"})
	if !hasDiagnostic(ds, "SEM047") {
		t.Fatalf("expected SEM047, got %v", ds)
	}
}

func TestVariantWhenUsesBaseSnapshot(t *testing.T) {
	p := project.New(".", "snapshot", []source.File{source.NewFile("snapshot.mosaic", []byte(`
module App {
    workload "main" { name = "app" image = "example.com/app:v1" replicas = 1 }
    extension workload.main.replicas
    extension workload.main.image
}
use App as app {}
variant scale { set app.workload.main.replicas = 3 }
variant baseGuard {
    when(eq(app.workload.main.replicas, 1)) {
        set app.workload.main.image = "example.com/app:base-observed"
    }
}
environment prod { use app apply scale apply baseGuard target kubernetes { namespace = "prod" } }
`))})
	result, ds := New(NewOptions{}).CompileInput(context.Background(), Input{RootProject: p, Environment: "prod"})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	image, _ := result.Graph.ReadField("application.app.workload.main", graph.FieldPath{"image"})
	got, _ := image.StringValue()
	if got != "example.com/app:base-observed" {
		t.Fatalf("image = %q", got)
	}
}

func TestWhenConditionDiagnostics(t *testing.T) {
	tests := []struct {
		condition string
		code      string
	}{
		{`input.value`, "SEM043"},
		{`missing(input.value)`, "SEM044"},
		{`gt(input.value, "bad")`, "SEM045"},
	}
	for _, test := range tests {
		p := project.New(".", "diagnostic", []source.File{source.NewFile("diagnostic.mosaic", []byte(`
module App(input: Input) { when(`+test.condition+`) { workload "main" { name = "app" image = "example.com/app:v1" } } }
use App as app { value = 1 }
environment prod { use app target kubernetes { namespace = "prod" } }
`))})
		_, ds := New(NewOptions{}).CompileInput(context.Background(), Input{RootProject: p, Environment: "prod"})
		if !hasDiagnostic(ds, test.code) {
			t.Fatalf("condition %s: expected %s, got %v", test.condition, test.code, ds)
		}
	}
}
