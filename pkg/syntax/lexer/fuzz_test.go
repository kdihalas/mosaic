package lexer_test

import (
	"bytes"
	"testing"

	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

var fuzzSeeds = [][]byte{
	{},
	[]byte(`module API(input: APIInput) { workload "api" {} }`),
	[]byte(`type Δ { message: string = "κόσμος" }`),
	[]byte(`/* outer /* inner */ outer */`),
	[]byte(`"unterminated`),
	[]byte(`/* unterminated`),
	{0xff, 0xfe, 'x'},
	[]byte(`1_ 1e+ 1_000._5`),
	{0xef, 0xbb, 0xbf, 'x'},
}

func FuzzLexNeverPanics(f *testing.F) {
	addSeeds(f)
	f.Fuzz(func(t *testing.T, input []byte) {
		_ = lexer.Lex(source.NewFile("fuzz.mosaic", input), lexer.Options{})
	})
}

func FuzzLexAlwaysReconstructsInput(f *testing.F) {
	addSeeds(f)
	f.Fuzz(func(t *testing.T, input []byte) {
		result := lexer.Lex(source.NewFile("fuzz.mosaic", input), lexer.Options{})
		var reconstructed []byte
		for _, tok := range result.Tokens {
			if tok.Kind != token.EOF {
				reconstructed = append(reconstructed, []byte(tok.Text)...)
			}
		}
		if !bytes.Equal(reconstructed, input) {
			t.Fatalf("reconstructed %x, want %x", reconstructed, input)
		}
	})
}

func FuzzLexAlwaysEndsWithEOF(f *testing.F) {
	addSeeds(f)
	f.Fuzz(func(t *testing.T, input []byte) {
		result := lexer.Lex(source.NewFile("fuzz.mosaic", input), lexer.Options{})
		count := 0
		for _, tok := range result.Tokens {
			if tok.Kind == token.EOF {
				count++
			}
		}
		if count != 1 || len(result.Tokens) == 0 || result.Tokens[len(result.Tokens)-1].Kind != token.EOF {
			t.Fatalf("EOF invariant failed: %#v", result.Tokens)
		}
	})
}

func FuzzLexMakesMonotonicProgress(f *testing.F) {
	addSeeds(f)
	f.Fuzz(func(t *testing.T, input []byte) {
		result := lexer.Lex(source.NewFile("fuzz.mosaic", input), lexer.Options{})
		previousEnd := 0
		for i, tok := range result.Tokens {
			if tok.Span.Start.Offset != previousEnd || tok.Span.End.Offset < tok.Span.Start.Offset || tok.Span.End.Offset > len(input) {
				t.Fatalf("token %d has invalid span %#v after %d", i, tok.Span, previousEnd)
			}
			if tok.Kind != token.EOF && tok.Span.End.Offset == tok.Span.Start.Offset {
				t.Fatalf("non-EOF token %d made no progress", i)
			}
			previousEnd = tok.Span.End.Offset
		}
		for i, diagnostic := range result.Diagnostics {
			if diagnostic.Span.Start.Offset < 0 || diagnostic.Span.End.Offset < diagnostic.Span.Start.Offset || diagnostic.Span.End.Offset > len(input) {
				t.Fatalf("diagnostic %d has invalid span %#v", i, diagnostic.Span)
			}
		}
	})
}

func addSeeds(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
}
