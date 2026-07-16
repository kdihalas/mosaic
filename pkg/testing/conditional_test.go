package testing_test

import (
	"context"
	"testing"

	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	mosaictesting "github.com/kdihalas/mosaic/pkg/testing"
)

func TestPresentInAssertions(t *testing.T) {
	p := project.New(".", "conditional-tests", []source.File{source.NewFile("tests.mosaic", []byte(`
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
test guarded { build prod assert reverse(present(app.resource.monitor)) }
test unguarded { build prod assert app.resource.monitor == null }
`))})
	report, ds := mosaictesting.Run(context.Background(), p, compiler.New(compiler.NewOptions{}))
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	if report.Passed != 1 || report.Failed != 1 {
		t.Fatalf("report = %#v", report)
	}
}
