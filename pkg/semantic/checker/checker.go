// Package checker provides the public semantic checking entry point.
package checker

import (
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/semantic"
)

type Options struct{ MaxDiagnostics int }

func Check(a *semantic.Analysis, _ Options) diagnostics.List {
	if a == nil {
		return diagnostics.List{{Code: "SEM000", Severity: diagnostics.SeverityError, Message: "analysis is nil"}}
	}
	return nil
}
