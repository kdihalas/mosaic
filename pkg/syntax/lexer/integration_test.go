package lexer_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

func TestExportedAPIGoldenFixtures(t *testing.T) {
	tests := []struct {
		input  string
		golden string
		format func(lexer.Result) string
	}{
		{"valid/module.mosaic", "golden/module.tokens", formatSignificant},
		{"invalid/malformed.mosaic", "golden/malformed.diagnostics", formatDiagnostics},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", test.input))
			if err != nil {
				t.Fatal(err)
			}
			golden, err := os.ReadFile(filepath.Join("testdata", test.golden))
			if err != nil {
				t.Fatal(err)
			}
			result := lexer.Lex(source.NewFile(test.input, content), lexer.Options{})
			if got := test.format(result); got != string(golden) {
				t.Fatalf("output mismatch\n--- got ---\n%s--- want ---\n%s", got, golden)
			}
			assertReconstructs(t, content, result)
		})
	}
}

func TestRepresentativeMosaicProgram(t *testing.T) {
	content := []byte(`type Scaling {
    minReplicas: int = 2
    require minReplicas >= 1
}
module WebApplication(input: Scaling) {
    workload "api" { replicas = input.minReplicas }
    export workload.api
}
variant production { set workload("api").replicas = 5 }
transform addMonitoring {
    select Workload where labels["monitoring"] != "disabled" {
        add port "metrics" { container = 9090 }
    }
}
environment productionEu {
    use api
    apply production
    resolve workload("api").replicas = 5
}
`)
	result := lexer.Lex(source.NewFile("representative.mosaic", content), lexer.Options{})
	if result.Diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
	assertReconstructs(t, content, result)
	wantKeywords := []token.Kind{
		token.KeywordType, token.KeywordRequire, token.KeywordModule, token.KeywordExport,
		token.KeywordVariant, token.KeywordTransform, token.KeywordSelect, token.KeywordWhere,
		token.KeywordEnvironment, token.KeywordUse, token.KeywordApply, token.KeywordResolve,
	}
	var gotKeywords []token.Kind
	for _, tok := range result.Tokens {
		if tok.Kind.IsKeyword() {
			gotKeywords = append(gotKeywords, tok.Kind)
		}
	}
	if fmt.Sprint(gotKeywords) != fmt.Sprint(wantKeywords) {
		t.Fatalf("keywords = %v, want %v", gotKeywords, wantKeywords)
	}
}

func formatSignificant(result lexer.Result) string {
	var output strings.Builder
	for _, tok := range token.Significant(result.Tokens) {
		fmt.Fprintf(&output, "%s %s\n", tok.Kind, strconv.Quote(tok.Text))
	}
	return output.String()
}

func formatDiagnostics(result lexer.Result) string {
	var output strings.Builder
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintf(&output, "%s %d:%d-%d:%d\n", diagnostic.Code,
			diagnostic.Span.Start.Line, diagnostic.Span.Start.Column,
			diagnostic.Span.End.Line, diagnostic.Span.End.Column)
	}
	return output.String()
}

func assertReconstructs(t *testing.T, input []byte, result lexer.Result) {
	t.Helper()
	var reconstructed []byte
	for _, tok := range result.Tokens {
		if tok.Kind != token.EOF {
			reconstructed = append(reconstructed, []byte(tok.Text)...)
		}
	}
	if !bytes.Equal(reconstructed, input) {
		t.Fatalf("tokens do not reconstruct input")
	}
}
