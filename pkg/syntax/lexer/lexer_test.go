package lexer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

func lexText(text string) Result {
	return Lex(source.NewFile("test.mosaic", []byte(text)), Options{})
}

func significantKinds(result Result) []token.Kind {
	tokens := token.Significant(result.Tokens)
	kinds := make([]token.Kind, len(tokens))
	for i, tok := range tokens {
		kinds[i] = tok.Kind
	}
	return kinds
}

func requireKinds(t *testing.T, result Result, want ...token.Kind) {
	t.Helper()
	got := significantKinds(result)
	if len(got) != len(want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kind[%d] = %s, want %s (all %v)", i, got[i], want[i], got)
		}
	}
}

func requireDiagnostic(t *testing.T, result Result, code string) diagnostics.Diagnostic {
	t.Helper()
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == code {
			return diagnostic
		}
	}
	t.Fatalf("missing diagnostic %s in %#v", code, result.Diagnostics)
	return diagnostics.Diagnostic{}
}

func TestEmptyAndPositionTracking(t *testing.T) {
	tests := []struct {
		name string
		text string
		line int
		col  int
	}{
		{"empty", "", 1, 1},
		{"lf", "a\nb", 2, 2},
		{"crlf", "a\r\nb", 2, 2},
		{"cr", "a\rb", 2, 2},
		{"unicode", "δε", 1, 3},
		{"tab", "\t", 1, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := lexText(test.text)
			eof := result.Tokens[len(result.Tokens)-1]
			if eof.Span.Start.Offset != len(test.text) || eof.Span.Start.Line != test.line || eof.Span.Start.Column != test.col || eof.Span.Start != eof.Span.End {
				t.Fatalf("EOF span = %#v", eof.Span)
			}
		})
	}

	result := lexText("α x")
	x := result.Tokens[2]
	if x.Span.Start.Offset != 3 || x.Span.Start.Column != 3 || x.Span.End.Column != 4 {
		t.Fatalf("Unicode position = %#v", x.Span)
	}
}

func TestBOMWhitespaceAndNewlines(t *testing.T) {
	result := Lex(source.NewFile("bom.mosaic", append([]byte{0xef, 0xbb, 0xbf}, []byte(" \t\r\n\r")...)), Options{})
	if result.Tokens[0].Kind != token.ByteOrderMark || result.Tokens[0].Span.Start.Column != 1 || result.Tokens[0].Span.End.Column != 1 {
		t.Fatalf("BOM token = %#v", result.Tokens[0])
	}
	if result.Tokens[1].Kind != token.Whitespace || result.Tokens[2].Text != "\r\n" || result.Tokens[3].Text != "\r" {
		t.Fatalf("trivia = %#v", result.Tokens)
	}

	misplaced := lexText("x\ufeffy")
	requireKinds(t, misplaced, token.Identifier, token.Invalid, token.Identifier, token.EOF)
	requireDiagnostic(t, misplaced, "LEX009")
}

func TestIdentifiersAndKeywords(t *testing.T) {
	result := lexText("api _api api2 productionEu serviceAccount Διαμόρφωση module Module moduleName true false null trueValue input workload")
	requireKinds(t, result,
		token.Identifier, token.Identifier, token.Identifier, token.Identifier, token.Identifier, token.Identifier,
		token.KeywordModule, token.Identifier, token.Identifier, token.True, token.False, token.Null,
		token.Identifier, token.Identifier, token.Identifier, token.EOF)
	values := token.Significant(result.Tokens)
	if values[6].Value != "" || values[9].Value != "true" || values[11].Value != "" || values[5].Value != "Διαμόρφωση" {
		t.Fatalf("identifier/literal values = %#v", values)
	}
}

func TestOperatorsAndLongestMatch(t *testing.T) {
	result := lexText("{}()[]:,.? = == ! != < <= > >= && || + - -> * / //x\n/*y*/")
	requireKinds(t, result,
		token.LeftBrace, token.RightBrace, token.LeftParen, token.RightParen, token.LeftBracket, token.RightBracket,
		token.Colon, token.Comma, token.Dot, token.Question, token.Equal, token.EqualEqual, token.Bang,
		token.BangEqual, token.Less, token.LessEqual, token.Greater, token.GreaterEqual, token.AndAnd,
		token.OrOr, token.Plus, token.Minus, token.Arrow, token.Star, token.Slash, token.EOF)
}

func TestComments(t *testing.T) {
	result := lexText("// one\r\n/* outer\n/* inner */ end */x")
	if result.Tokens[0].Kind != token.LineComment || result.Tokens[1].Kind != token.Newline || result.Tokens[2].Kind != token.BlockComment {
		t.Fatalf("comment tokens = %#v", result.Tokens)
	}
	if result.Tokens[2].Span.End.Line != 3 {
		t.Fatalf("block comment span = %#v", result.Tokens[2].Span)
	}
	unterminated := lexText("/* missing")
	requireKinds(t, unterminated, token.Invalid, token.EOF)
	requireDiagnostic(t, unterminated, "LEX007")
}

func TestValidNumbers(t *testing.T) {
	result := lexText("0 42 1_000 999999999999999999999999999999999999 1.0 1e3 1E+3 1e-3 1_000.50 1.5e6")
	wantKinds := []token.Kind{token.Integer, token.Integer, token.Integer, token.Integer, token.Decimal, token.Decimal, token.Decimal, token.Decimal, token.Decimal, token.Decimal, token.EOF}
	requireKinds(t, result, wantKinds...)
	significant := token.Significant(result.Tokens)
	if significant[2].Value != "1000" || significant[8].Value != "1000.50" {
		t.Fatalf("normalized values = %#v", significant)
	}
}

func TestMalformedAndBoundaryNumbers(t *testing.T) {
	for _, text := range []string{"1_", "1__0", "1e", "1e+", "1e-", "1.5_", "1_000._5"} {
		t.Run(text, func(t *testing.T) {
			result := lexText(text)
			requireKinds(t, result, token.Invalid, token.EOF)
			requireDiagnostic(t, result, "LEX008")
		})
	}

	tests := []struct {
		text string
		want []token.Kind
	}{
		{"1.", []token.Kind{token.Integer, token.Dot, token.EOF}},
		{".5", []token.Kind{token.Dot, token.Integer, token.EOF}},
		{"1._5", []token.Kind{token.Integer, token.Dot, token.Identifier, token.EOF}},
		{"123abc", []token.Kind{token.Integer, token.Identifier, token.EOF}},
		{"-42", []token.Kind{token.Minus, token.Integer, token.EOF}},
	}
	for _, test := range tests {
		requireKinds(t, lexText(test.text), test.want...)
	}
}

func TestValidStrings(t *testing.T) {
	tests := map[string]string{
		`""`: "", `"api"`: "api", `"hello world"`: "hello world", `"hello\nworld"`: "hello\nworld",
		`"quote: \""`: `quote: "`, `"backslash: \\"`: `backslash: \`, `"\u03BB"`: "λ",
		`"\U0001F600"`: "😀", `"Καλημέρα"`: "Καλημέρα", `"\0\b\f\r\t"`: "\x00\b\f\r\t",
	}
	for text, want := range tests {
		t.Run(text, func(t *testing.T) {
			result := lexText(text)
			requireKinds(t, result, token.String, token.EOF)
			if got := result.Tokens[0].Value; got != want {
				t.Fatalf("value = %q, want %q", got, want)
			}
		})
	}
}

func TestInvalidStringsAndRecovery(t *testing.T) {
	tests := []struct{ text, code string }{
		{`"bad\qescape" next`, "LEX004"}, {`"\u12" next`, "LEX005"}, {`"\uZZZZ"`, "LEX005"},
		{`"\U00110000"`, "LEX005"}, {`"\uD800"`, "LEX005"}, {"\"raw\x01control\"", "LEX006"},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			result := lexText(test.text)
			if result.Tokens[0].Kind != token.Invalid || result.Tokens[0].Value != "" {
				t.Fatalf("invalid string token = %#v", result.Tokens[0])
			}
			requireDiagnostic(t, result, test.code)
		})
	}

	newline := lexText("\"raw newline\nnext")
	if newline.Tokens[0].Kind != token.Invalid || newline.Tokens[1].Kind != token.Newline || newline.Tokens[0].Text != "\"raw newline" {
		t.Fatalf("newline recovery = %#v", newline.Tokens)
	}
	requireDiagnostic(t, newline, "LEX003")

	eof := lexText(`"unterminated`)
	requireKinds(t, eof, token.Invalid, token.EOF)
	requireDiagnostic(t, eof, "LEX003")
}

func TestUnexpectedInvalidUTF8AndRecovery(t *testing.T) {
	result := lexText("name = @api")
	requireKinds(t, result, token.Identifier, token.Equal, token.Invalid, token.Identifier, token.EOF)
	requireDiagnostic(t, result, "LEX001")

	bytesInput := []byte{'x', 0xff, 0xfe, 'y'}
	invalid := Lex(source.NewFile("bytes.mosaic", bytesInput), Options{})
	requireKinds(t, invalid, token.Identifier, token.Invalid, token.Invalid, token.Identifier, token.EOF)
	if invalid.Tokens[1].Text != string([]byte{0xff}) || invalid.Tokens[2].Span.Start.Column != 3 {
		t.Fatalf("invalid byte tokens = %#v", invalid.Tokens)
	}
	if len(invalid.Diagnostics) != 2 || invalid.Diagnostics[0].Code != "LEX002" || invalid.Diagnostics[1].Code != "LEX002" {
		t.Fatalf("invalid byte diagnostics = %#v", invalid.Diagnostics)
	}

	inside := Lex(source.NewFile("string.mosaic", []byte{'"', 'a', 0xff, 'b', '"'}), Options{})
	requireKinds(t, inside, token.Invalid, token.EOF)
	requireDiagnostic(t, inside, "LEX002")
}

func TestDiagnosticLimitAtExactLimit(t *testing.T) {
	result := Lex(source.NewFile("limit.mosaic", []byte("@@@")), Options{MaxDiagnostics: 2})
	if len(result.Diagnostics) != 3 || result.Diagnostics[0].Code != "LEX001" || result.Diagnostics[1].Code != "LEX001" || result.Diagnostics[2].Code != "LEX010" {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	if result.Diagnostics[2].Span.Start != result.Diagnostics[1].Span.End || result.Diagnostics[2].Span.Start != result.Diagnostics[2].Span.End {
		t.Fatalf("limit span = %#v", result.Diagnostics[2].Span)
	}
}

func TestRoundTripAndDeterminism(t *testing.T) {
	inputs := [][]byte{
		{}, []byte("\xef\xbb\xbfmodule API(input: APIInput) {\r\n // x\n}\r"),
		[]byte("Δ = \"κόσμος\\n\" /* nested /* yes */ */ 1_000.5e-2"),
		{'"', 'x', 0xff, '"', '\n', '/', '*'},
	}
	for _, input := range inputs {
		first := Lex(source.NewFile("roundtrip.mosaic", input), Options{})
		second := Lex(source.NewFile("roundtrip.mosaic", input), Options{})
		var reconstructed strings.Builder
		for _, tok := range first.Tokens[:len(first.Tokens)-1] {
			reconstructed.WriteString(tok.Text)
		}
		if !bytes.Equal([]byte(reconstructed.String()), input) {
			t.Fatalf("reconstruction = %q, want %q", reconstructed.String(), input)
		}
		if len(first.Tokens) != len(second.Tokens) || len(first.Diagnostics) != len(second.Diagnostics) {
			t.Fatal("nondeterministic result sizes")
		}
		for i := range first.Tokens {
			if first.Tokens[i] != second.Tokens[i] {
				t.Fatalf("token %d differs", i)
			}
		}
	}
}
