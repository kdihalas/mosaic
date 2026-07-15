package compiler

import (
	"context"
	"testing"

	"github.com/kdihalas/mosaic/pkg/graph"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/project"
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

func TestPackagePrivateModuleRejected(t *testing.T) {
	root := project.New(".", "catalog", []source.File{source.NewFile("root.mosaic", []byte(`use http.PrivateModule as app {}`))})
	pkg := CompilationPackage{Identity: "acme/http", Version: "1.0.0", Aliases: []string{"http"}, Manifest: mosaicpackage.Manifest{Name: "acme/http", Version: "1.0.0"}, Files: []source.File{source.NewFile("p.mosaic", []byte(`module PrivateModule {}`))}}
	_, ds := New(NewOptions{}).AnalyzeInput(context.Background(), Input{RootProject: root, Packages: []CompilationPackage{pkg}})
	if !ds.HasErrors() || ds[0].Code != "PKG031" {
		t.Fatalf("expected PKG031: %v", ds)
	}
}
