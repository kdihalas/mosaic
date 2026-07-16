package bundle

import (
	"context"
	"strings"
	"testing"

	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

func TestSchemaRecordsAbsentOptionalExport(t *testing.T) {
	p := project.New(".", "optional", []source.File{source.NewFile("optional.mosaic", []byte(`
module App(input: Input) {
    workload "main" { name = "app" image = "example.com/app:v1" }
    export workload.main
    when(input.monitoring) {
        resource "monitor" { apiVersion = "example.com/v1" kind = "Monitor" name = "app" }
        export resource.monitor
    }
}
use App as app { monitoring = false }
environment prod { use app target kubernetes { namespace = "prod" } }
`))})
	result, ds := compiler.New(compiler.NewOptions{}).CompileInput(context.Background(), compiler.Input{RootProject: p, Environment: "prod"})
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	built, err := Build(result, renderer.ArtifactSet{}, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	schema := string(built.Files["schema.json"])
	for _, want := range []string{`"id": "application.app.resource.monitor"`, `"optional": true`, `"present": false`} {
		if !strings.Contains(schema, want) {
			t.Fatalf("missing %s in:\n%s", want, schema)
		}
	}
}
