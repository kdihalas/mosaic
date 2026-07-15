package token_test

import (
	"strings"
	"testing"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

func TestEveryKindHasStableName(t *testing.T) {
	for kind := token.EOF; kind <= token.KeywordAs; kind++ {
		if got := kind.String(); got == "" || strings.HasPrefix(got, "Kind(") {
			t.Errorf("kind %d has invalid name %q", kind, got)
		}
	}
	if got := token.Kind(999).String(); got != "Kind(999)" {
		t.Fatalf("unknown kind name = %q", got)
	}
}

func TestClassifications(t *testing.T) {
	checks := []struct {
		name string
		got  bool
	}{
		{"trivia", token.Newline.IsTrivia()},
		{"keyword", token.KeywordModule.IsKeyword()},
		{"literal", token.Decimal.IsLiteral()},
		{"operator", token.Arrow.IsOperator()},
		{"delimiter", token.LeftBrace.IsDelimiter()},
	}
	for _, check := range checks {
		if !check.got {
			t.Errorf("%s classification is false", check.name)
		}
	}
	if token.Identifier.IsKeyword() || token.Invalid.IsTrivia() || token.Comma.IsOperator() {
		t.Fatal("unrelated kind was misclassified")
	}
}

func TestLookupKeyword(t *testing.T) {
	tests := map[string]token.Kind{
		"module": token.KeywordModule, "Module": token.Identifier,
		"moduleName": token.Identifier, "true": token.True,
		"trueValue": token.Identifier, "null": token.Null,
		"input": token.Identifier, "workload": token.Identifier,
		"set": token.Identifier, "target": token.Identifier,
	}
	for text, want := range tests {
		if got := token.LookupKeyword(text); got != want {
			t.Errorf("LookupKeyword(%q) = %s, want %s", text, got, want)
		}
	}
}

func TestAllHardKeywords(t *testing.T) {
	words := []string{
		"type", "enum", "module", "resource", "variant", "environment", "transform", "policy", "test",
		"use", "apply", "enable", "select", "where", "resolve", "require", "deny", "warn", "export",
		"protected", "extension", "for", "in", "if", "else", "as",
	}
	for index, word := range words {
		want := token.Kind(int(token.KeywordType) + index)
		if got := token.LookupKeyword(word); got != want || !got.IsKeyword() {
			t.Errorf("LookupKeyword(%q) = %s, want %s", word, got, want)
		}
	}
}

func TestSignificantAndFormat(t *testing.T) {
	span := diagnostics.Span{Start: diagnostics.Position{Line: 1, Column: 1}, End: diagnostics.Position{Line: 2, Column: 1}}
	tokens := []token.Token{{Kind: token.Newline, Text: "\r\n", Span: span}, {Kind: token.Invalid}, {Kind: token.EOF}}
	got := token.Significant(tokens)
	if len(got) != 2 || got[0].Kind != token.Invalid || got[1].Kind != token.EOF {
		t.Fatalf("Significant() = %#v", got)
	}
	if got := token.Format(tokens[0]); got != `Newline "\r\n" 1:1-2:1` {
		t.Fatalf("Format() = %q", got)
	}
}
