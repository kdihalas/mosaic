package semantic_test

import (
	"github.com/kdihalas/mosaic/pkg/semantic"
	"github.com/kdihalas/mosaic/pkg/syntax/ast"
	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/parser"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/kdihalas/mosaic/pkg/value"
	"testing"
)

func TestListIndexExpression(t *testing.T) {
	src := source.NewFile("test.mosaic", []byte(`test x { assert values[1] == "second" }`))
	l := lexer.Lex(src, lexer.Options{})
	p := parser.Parse(src, l.Tokens, parser.Options{})
	if p.Diagnostics.HasErrors() {
		t.Fatalf("%#v", p.Diagnostics)
	}
	decl := p.File.Declarations[0].(*ast.TestDeclaration)
	op := decl.Body[0].(*ast.OperationStatement)
	v, err := semantic.Evaluate(op.Value, semantic.Context{Values: map[string]value.Value{"values": value.List([]value.Value{value.String("first"), value.String("second")})}})
	if err != nil {
		t.Fatal(err)
	}
	ok, _ := v.BoolValue()
	if !ok {
		t.Fatal("list index did not select the second value")
	}
}

func TestPathSupportsQuotedStringKeys(t *testing.T) {
	src := source.NewFile("test.mosaic", []byte(`environment prod {
    resolve app.workload.main.labels["app.example.com/tier"] = "local"
}`))
	l := lexer.Lex(src, lexer.Options{})
	p := parser.Parse(src, l.Tokens, parser.Options{})
	if p.Diagnostics.HasErrors() {
		t.Fatalf("%#v", p.Diagnostics)
	}
	decl := p.File.Declarations[0].(*ast.EnvironmentDeclaration)
	op := decl.Body[0].(*ast.ResolveStatement)
	path, ok := semantic.Path(op.Target)
	if !ok {
		t.Fatal("quoted string index was not accepted as a path segment")
	}
	want := []string{"app", "workload", "main", "labels", "app.example.com/tier"}
	if len(path) != len(want) {
		t.Fatalf("path = %#v, want %#v", path, want)
	}
	for i := range want {
		if path[i] != want[i] {
			t.Fatalf("path = %#v, want %#v", path, want)
		}
	}
}
