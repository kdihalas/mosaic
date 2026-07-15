package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kdihalas/mosaic/pkg/bundle"
	"github.com/kdihalas/mosaic/pkg/graph"
)

func TestRunProjectWithSelectedVariant(t *testing.T) {
	root := t.TempDir()
	write(t, root, "mosaic.toml", []byte("name = \"catalog\"\nlanguage_version = \"v1alpha1\"\nsources = [\"**/*.mosaic\"]\n"))
	write(t, root, "app.mosaic", []byte(`
module App {
    workload "main" { name = "catalog" image = "example.com/catalog:v1" replicas = 1 }
    extension workload.main.replicas
}
use App as catalog {}
variant production { set catalog.workload.main.replicas = 3 }
environment prod { use catalog target kubernetes { namespace = "catalog" } }
`))

	result, ds := Run(context.Background(), Input{
		RootPath: root, InputKind: InputKindAuto, Environment: "prod",
		Variants: []string{"production"}, Offline: true, Locked: true,
	})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	if result.InputKind != InputKindProject {
		t.Fatalf("kind = %s", result.InputKind)
	}
	v, ok := result.Compilation.Graph.ReadField("application.catalog.workload.main", graph.FieldPath{"replicas"})
	if !ok {
		t.Fatal("missing replicas")
	}
	i, _ := v.IntValue()
	if got := i.Int64(); got != 3 {
		t.Fatalf("replicas = %d", got)
	}
	if len(result.Rendered.Files["kubernetes.yaml"]) == 0 || result.Bundle == nil {
		t.Fatal("missing rendered or bundle output")
	}
}

func TestRunDerivesBundleWithBakedInVariant(t *testing.T) {
	root := t.TempDir()
	write(t, root, "mosaic.toml", []byte("name = \"catalog\"\nlanguage_version = \"v1alpha1\"\nsources = [\"**/*.mosaic\"]\n"))
	write(t, root, "app.mosaic", []byte(`
module App {
    workload "main" { name = "catalog" image = "example.com/catalog:v1" replicas = 1 }
    extension workload.main.replicas
}
use App as catalog {}
variant production { set catalog.workload.main.replicas = 3 }
environment prod { use catalog target kubernetes { namespace = "catalog" } }
`))
	base, ds := Run(context.Background(), Input{
		RootPath: root, InputKind: InputKindProject, Environment: "prod", Offline: true, Locked: true,
	})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	if err := bundle.WriteDirectory(context.Background(), base.Bundle, bundleRoot); err != nil {
		t.Fatal(err)
	}
	derived, ds := Run(context.Background(), Input{
		RootPath: bundleRoot, InputKind: InputKindBundle, Environment: "prod",
		Variants: []string{"production"}, Offline: true, Locked: true,
	})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	v, ok := derived.Compilation.Graph.ReadField("application.catalog.workload.main", graph.FieldPath{"replicas"})
	if !ok {
		t.Fatal("missing replicas")
	}
	i, _ := v.IntValue()
	if got := i.Int64(); got != 3 {
		t.Fatalf("replicas = %d", got)
	}
	for _, name := range []string{"schema.json", "extension-points.json", "build-recipe.json"} {
		if _, ok := derived.Bundle.Files[name]; !ok {
			t.Fatalf("missing %s", name)
		}
	}
}

func TestRunPackageRequiresExportedEnvironment(t *testing.T) {
	root := t.TempDir()
	write(t, root, "mosaic.package.toml", []byte(`
name = "acme/catalog"
version = "1.0.0"
language_version = "v1alpha1"
sources = ["*.mosaic"]
[exports]
modules = ["App"]
`))
	write(t, root, "app.mosaic", []byte(`
module App { workload "main" { name = "catalog" image = "example.com/catalog:v1" } }
use App as catalog {}
environment prod { use catalog target kubernetes { namespace = "catalog" } }
`))

	_, ds := Run(context.Background(), Input{
		RootPath: root, InputKind: InputKindPackage, Environment: "prod", Offline: true, Locked: true,
	})
	if !ds.HasErrors() || ds[0].Code != "BLD003" {
		t.Fatalf("expected BLD003, got %v", ds)
	}
}

func TestDetectInputKindRejectsAmbiguousLayout(t *testing.T) {
	root := t.TempDir()
	write(t, root, "mosaic.toml", nil)
	write(t, root, "mosaic.package.toml", nil)
	_, ds := DetectInputKind(root, InputKindAuto)
	if !ds.HasErrors() || ds[0].Code != "BLD002" {
		t.Fatalf("expected BLD002, got %v", ds)
	}
}

func TestRunRequiresOfflineLockedMode(t *testing.T) {
	_, ds := Run(context.Background(), Input{RootPath: t.TempDir(), Environment: "prod"})
	if !ds.HasErrors() || ds[0].Code != "BLD001" {
		t.Fatalf("expected BLD001, got %v", ds)
	}
}

func write(t *testing.T, root, name string, content []byte) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
