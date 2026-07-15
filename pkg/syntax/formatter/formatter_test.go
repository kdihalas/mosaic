package formatter_test

import (
	"bytes"
	"github.com/kdihalas/mosaic/pkg/syntax/formatter"
	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/parser"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"testing"
)

func TestIdempotentAndComments(t *testing.T) {
	raw := []byte("module M(input:T) {\n// keep\n workload \"x\" { labels { \"a/b\"=\"c\" } }\n}\n")
	format := func(b []byte) []byte {
		s := source.NewFile("x", b)
		l := lexer.Lex(s, lexer.Options{})
		p := parser.Parse(s, l.Tokens, parser.Options{})
		if p.Diagnostics.HasErrors() {
			t.Fatalf("%#v", p.Diagnostics)
		}
		return append(formatter.Format(p.File), '\n')
	}
	a := format(raw)
	b := format(a)
	if !bytes.Equal(a, b) {
		t.Fatalf("not idempotent\n%s\n%s", a, b)
	}
	if !bytes.Contains(a, []byte("// keep")) || !bytes.Contains(a, []byte(`"a/b"`)) {
		t.Fatal("lost comment or quoted key")
	}
}
