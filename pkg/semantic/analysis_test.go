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

func TestPredicateFunctions(t *testing.T) {
	tests := []struct {
		expression string
		want       bool
	}{
		{`gt(3, 2.5)`, true},
		{`lt(2, 3)`, true},
		{`eq(2, 2.0)`, true},
		{`includes("api", ["web", "api"])`, true},
		{`has("owner", { owner = "team" })`, true},
		{`empty([])`, true},
		{`zero(0.0)`, true},
		{`any(false, false, true)`, true},
		{`both(true, true, true)`, true},
		{`reverse(false)`, true},
	}
	for _, test := range tests {
		f := source.NewFile("predicate.mosaic", []byte(`test predicate { assert `+test.expression+` }`))
		l := lexer.Lex(f, lexer.Options{})
		p := parser.Parse(f, l.Tokens, parser.Options{})
		if p.Diagnostics.HasErrors() {
			t.Fatalf("%s: %v", test.expression, p.Diagnostics)
		}
		op := p.File.Declarations[0].(*ast.TestDeclaration).Body[0].(*ast.OperationStatement)
		got, err := semantic.Evaluate(op.Value, semantic.Context{})
		if err != nil {
			t.Fatalf("%s: %v", test.expression, err)
		}
		value, ok := got.BoolValue()
		if !ok || value != test.want {
			t.Fatalf("%s = %v, want %v", test.expression, got, test.want)
		}
	}
}

func TestBooleanFunctionsShortCircuit(t *testing.T) {
	f := source.NewFile("short.mosaic", []byte(`test predicate { assert both(false, unknown()) == false }`))
	l := lexer.Lex(f, lexer.Options{})
	p := parser.Parse(f, l.Tokens, parser.Options{})
	op := p.File.Declarations[0].(*ast.TestDeclaration).Body[0].(*ast.OperationStatement)
	if _, err := semantic.Evaluate(op.Value, semantic.Context{}); err != nil {
		t.Fatal(err)
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
