package parser_test

import (
	"strings"
	"testing"

	"github.com/kdihalas/mosaic/pkg/semantic"
	"github.com/kdihalas/mosaic/pkg/syntax/ast"
	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/parser"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

func parse(t *testing.T, s string) *ast.File {
	t.Helper()
	f := source.NewFile("test.mosaic", []byte(s))
	l := lexer.Lex(f, lexer.Options{})
	r := parser.Parse(f, l.Tokens, parser.Options{})
	if r.Diagnostics.HasErrors() {
		t.Fatalf("%#v", r.Diagnostics)
	}
	return r.File
}
func TestDeclarationsAndGenericTypes(t *testing.T) {
	f := parse(t, `type Input { labels: map<string, string> = {} }
enum Mode { dev, prod }
module M(input: Input) { workload "main" { image = input.image } }
use M as app { labels = {} }
variant v { set app.workload.main.replicas = 2 }
environment e { use app apply v }
transform x { select Workload { set app.workload.main.replicas = 2 } }
policy p { select Workload { require replicas > 0 { message = "positive" } } }
test q { build e assert app.workload.main.replicas == 2 }
`)
	if len(f.Declarations) != 9 {
		t.Fatalf("declarations=%d", len(f.Declarations))
	}
}

func TestCustomResourceDeclaration(t *testing.T) {
	f := parse(t, `module M(input: Input) { resource "monitor" { apiVersion = "monitoring.coreos.com/v1" kind = "ServiceMonitor" name = input.name spec { endpoints = [{ port = "metrics" }] } } }`)
	m := f.Declarations[0].(*ast.ModuleDeclaration)
	r, ok := m.Body[0].(*ast.ResourceDeclaration)
	if !ok || r.Kind != "resource" || r.Name != "monitor" {
		t.Fatalf("%#v", m.Body[0])
	}
}
func TestExpressionPrecedence(t *testing.T) {
	f := parse(t, `test q { assert true || false && 1 + 2 * 3 == 7 }`)
	d := f.Declarations[0].(*ast.TestDeclaration)
	o := d.Body[0].(*ast.OperationStatement)
	root := o.Value.(*ast.BinaryExpression)
	if root.Operator != "||" {
		t.Fatalf("root=%s", root.Operator)
	}
}
func TestQuotedObjectKeysAndRecovery(t *testing.T) {
	parse(t, `use M as a { labels = { "app.kubernetes.io/name" = "a" } }`)
	f := source.NewFile("bad.mosaic", []byte(`module M(input: T) { workload "x" { image = } } environment ok {}`))
	l := lexer.Lex(f, lexer.Options{})
	r := parser.Parse(f, l.Tokens, parser.Options{})
	if !r.Diagnostics.HasErrors() || len(r.File.Declarations) < 1 {
		t.Fatal("expected recovery")
	}
}

func TestQualifiedPackageNames(t *testing.T) {
	f := parse(t, `use http.HttpService as catalog {}
type Settings { profile: http.ResourceProfile }
environment prod { use catalog apply http.productionDefaults policies { use http.requiredResources } }
`)
	use := f.Declarations[0].(*ast.ModuleUseDeclaration)
	if use.Module != "http.HttpService" {
		t.Fatalf("module = %q", use.Module)
	}
	typ := f.Declarations[1].(*ast.TypeDeclaration)
	path, ok := semantic.Path(typ.Fields[0].Type)
	if !ok || strings.Join(path, ".") != "http.ResourceProfile" {
		t.Fatalf("type path = %v", path)
	}
}

func TestWhenStatement(t *testing.T) {
	f := parse(t, `module M(input: Input) {
    when(both(input.enabled, reverse(empty(input.name)))) {
        workload "main" { name = input.name }
    }
}`)
	module := f.Declarations[0].(*ast.ModuleDeclaration)
	guard, ok := module.Body[0].(*ast.WhenStatement)
	if !ok || len(guard.Body) != 1 {
		t.Fatalf("guard = %#v", module.Body[0])
	}
}

func TestCapabilityDeclarationAndQualifiedEnable(t *testing.T) {
	f := parse(t, `
capability Monitor(input: MonitorInput) { resource "monitor" { kind = "Monitor" } }
variant production {
    enable observability.Monitor as metrics {
        target = app.workload.main
    }
}`)
	capability := f.Declarations[0].(*ast.CapabilityDeclaration)
	if capability.Name != "Monitor" || capability.Parameter == nil || capability.Parameter.Name != "input" {
		t.Fatalf("capability = %#v", capability)
	}
	variant := f.Declarations[1].(*ast.VariantDeclaration)
	op := (*ast.OperationStatement)(variant.Body[0].(*ast.EnableStatement))
	if op.Name != "observability.Monitor" || op.Identity != "metrics" {
		t.Fatalf("enable = %#v", op)
	}
}
func FuzzParserNeverPanics(f *testing.F) {
	f.Add([]byte(`environment dev {}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		s := source.NewFile("fuzz.mosaic", b)
		l := lexer.Lex(s, lexer.Options{MaxDiagnostics: 20})
		_ = parser.Parse(s, l.Tokens, parser.Options{MaxDiagnostics: 20, MaxParseDepth: 32, MaxExpressionDepth: 32})
	})
}
