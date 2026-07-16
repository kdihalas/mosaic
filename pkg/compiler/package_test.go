package compiler

import (
	"context"
	"strings"
	"testing"

	"github.com/kdihalas/mosaic/pkg/graph"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/renderer/kubernetes"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

func TestPackageNamespaceModuleVariantAndPolicy(t *testing.T) {
	root := project.New(".", "catalog", []source.File{source.NewFile("root.mosaic", []byte(`
use http.HttpService as catalog { name = "catalog" image = "ghcr.io/acme/catalog:1.0.0" }
environment prod {
    use catalog
    apply http.productionDefaults
    policies { use http.requiredResources }
    target kubernetes { namespace = "prod" }
}
`))})
	pkg := CompilationPackage{
		Identity: "acme/http", Version: "1.0.0", Aliases: []string{"http"},
		Manifest: mosaicpackage.Manifest{Name: "acme/http", Version: "1.0.0"},
		Exports:  mosaicpackage.ExportSet{Modules: []string{"HttpService"}, Variants: []string{"productionDefaults"}, Policies: []string{"requiredResources"}},
		Files: []source.File{source.NewFile("src/package.mosaic", []byte(`
module HttpService {
    workload "main" { name = input.name image = input.image replicas = 1 }
    extension workload.main.replicas
}

variant productionDefaults { set catalog.workload.main.replicas = 3 }
policy requiredResources { select Workload { deny replicas < 2 { message = "resources required" } } }
module PrivateModule {}
`))},
	}
	result, ds := New(NewOptions{}).CompileInput(context.Background(), Input{RootProject: root, Packages: []CompilationPackage{pkg}, Environment: "prod"})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	v, ok := result.Graph.ReadField(graph.ResourceID("application.catalog.workload.main"), graph.FieldPath{"replicas"})
	if !ok {
		t.Fatal("missing replicas")
	}
	i, _ := v.IntValue()
	if i.Int64() != 3 {
		t.Fatalf("replicas = %s", i)
	}
	if len(result.PolicyReport.Results) != 0 {
		t.Fatalf("policy unexpectedly failed: %#v", result.PolicyReport)
	}
	if result.Instances[0].Module != "package.acme/http@1.0.0.module.HttpService" {
		t.Fatalf("module ID = %q", result.Instances[0].Module)
	}
}

func TestPackageDefinedCapabilityExpansion(t *testing.T) {
	root := project.New(".", "catalog", []source.File{source.NewFile("root.mosaic", []byte(`
module App {
    workload "main" { name = "catalog" image = "example.com/catalog:v1" }
}
use App as app {}
variant monitored {
    enable obs.Monitor as metrics {
        target = app.workload.main
        name = "catalog-metrics"
        enabled = true
        interval = "15s"
    }
}
environment prod { use app apply monitored target kubernetes { namespace = "prod" } }
`))})
	pkg := CompilationPackage{
		Identity: "acme/observability", Version: "1.0.0", Aliases: []string{"obs"},
		Manifest: mosaicpackage.Manifest{Name: "acme/observability", Version: "1.0.0"},
		Exports:  mosaicpackage.ExportSet{Capabilities: []string{"Monitor"}, Types: []string{"MonitorInput"}},
		Files: []source.File{source.NewFile("src/package.mosaic", []byte(`
type MonitorInput {
    target: reference<Workload>
    name: ResourceName
    enabled: bool = true
    interval: Duration = "30s"
}
capability Monitor(input: MonitorInput) {
    when(input.enabled) {
        resource "monitor" {
            apiVersion = "monitoring.coreos.com/v1"
            kind = "ServiceMonitor"
            name = input.name
            spec { endpoints = [{ port = "metrics", interval = input.interval }] }
        }
        export resource.monitor
        extension resource.monitor.spec.endpoints
    }
}
`))},
	}
	result, ds := New(NewOptions{}).CompileInput(context.Background(), Input{RootProject: root, Packages: []CompilationPackage{pkg}, Environment: "prod"})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	id := graph.ResourceID("application.app.capability.metrics.resource.monitor")
	resource, ok := result.Graph.Get(id)
	if !ok || resource.Type != "kubernetes.CustomResource" || resource.Name != "catalog-metrics" {
		t.Fatalf("expanded resource = %#v, found %v", resource, ok)
	}
	if len(result.Capabilities) != 1 || result.Capabilities[0].Capability != "package.acme/observability@1.0.0.capability.Monitor" {
		t.Fatalf("capability instances = %#v", result.Capabilities)
	}
	if len(result.Capabilities[0].Exports) != 1 || !result.Capabilities[0].Exports[0].Optional || !result.Capabilities[0].Exports[0].Present {
		t.Fatalf("capability exports = %#v", result.Capabilities[0].Exports)
	}
	rendered, rds := kubernetes.New().Render(context.Background(), renderer.RenderInput{Environment: "prod", Graph: result.Graph, Provenance: result.Provenance, Options: result.Metadata.TargetOptions})
	if rds.HasErrors() {
		t.Fatal(rds)
	}
	if yaml := string(rendered.Files["kubernetes.yaml"]); !strings.Contains(yaml, "kind: ServiceMonitor") || !strings.Contains(yaml, "interval: 15s") {
		t.Fatalf("capability resource not rendered:\n%s", yaml)
	}
}

func TestPackagePrivateModuleRejected(t *testing.T) {
	root := project.New(".", "catalog", []source.File{source.NewFile("root.mosaic", []byte(`use http.PrivateModule as app {}`))})
	pkg := CompilationPackage{Identity: "acme/http", Version: "1.0.0", Aliases: []string{"http"}, Manifest: mosaicpackage.Manifest{Name: "acme/http", Version: "1.0.0"}, Files: []source.File{source.NewFile("p.mosaic", []byte(`module PrivateModule {}`))}}
	_, ds := New(NewOptions{}).AnalyzeInput(context.Background(), Input{RootProject: root, Packages: []CompilationPackage{pkg}})
	if !ds.HasErrors() || ds[0].Code != "PKG031" {
		t.Fatalf("expected PKG031: %v", ds)
	}
}
