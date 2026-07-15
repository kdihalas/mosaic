// Package resolver exposes deterministic symbol collection for parsed files.
package resolver

import (
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/semantic"
	"github.com/kdihalas/mosaic/pkg/semantic/symbols"
	"github.com/kdihalas/mosaic/pkg/syntax/ast"
)

func Resolve(files []*ast.File) (*semantic.Analysis, diagnostics.List) {
	t := symbols.New()
	var ds diagnostics.List
	for _, f := range files {
		for _, d := range f.Declarations {
			var k symbols.Kind
			var n string
			switch x := d.(type) {
			case *ast.TypeDeclaration:
				k, n = symbols.Type, x.Name
			case *ast.EnumDeclaration:
				k, n = symbols.Enum, x.Name
			case *ast.ModuleDeclaration:
				k, n = symbols.Module, x.Name
			case *ast.ModuleUseDeclaration:
				k, n = symbols.Application, x.Alias
			case *ast.VariantDeclaration:
				k, n = symbols.Variant, x.Name
			case *ast.EnvironmentDeclaration:
				k, n = symbols.Environment, x.Name
			case *ast.TransformDeclaration:
				k, n = symbols.Transform, x.Name
			case *ast.PolicyDeclaration:
				k, n = symbols.Policy, x.Name
			case *ast.TestDeclaration:
				k, n = symbols.Test, x.Name
			}
			if n != "" && !t.Add(symbols.Symbol{ID: string(k) + "." + n, Name: n, Kind: k, Source: d.Span()}) {
				ds = append(ds, diagnostics.Diagnostic{Code: "SEM001", Severity: diagnostics.SeverityError, Message: "duplicate declaration `" + n + "`", Span: d.Span()})
			}
		}
	}
	return &semantic.Analysis{Files: files, Symbols: t.List()}, ds.Sorted()
}
