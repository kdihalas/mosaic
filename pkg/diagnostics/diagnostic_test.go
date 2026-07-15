package diagnostics_test

import (
	"testing"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
)

func TestListHasErrors(t *testing.T) {
	warning := diagnostics.Diagnostic{Severity: diagnostics.SeverityWarning}
	err := diagnostics.Diagnostic{Severity: diagnostics.SeverityError}
	if (diagnostics.List{warning}).HasErrors() {
		t.Fatal("warning-only list reported errors")
	}
	if !(diagnostics.List{warning, err}).HasErrors() {
		t.Fatal("error was not detected")
	}
}
