package token

import (
	"fmt"
	"strconv"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
)

// Token is one exact lexical slice and its optional decoded value.
type Token struct {
	Kind  Kind             `json:"kind"`
	Text  string           `json:"text"`
	Value string           `json:"value"`
	Span  diagnostics.Span `json:"span"`
}

// Significant returns a copy of tokens with trivia removed. Invalid and EOF
// tokens are retained.
func Significant(tokens []Token) []Token {
	result := make([]Token, 0, len(tokens))
	for _, tok := range tokens {
		if !tok.Kind.IsTrivia() {
			result = append(result, tok)
		}
	}
	return result
}

// Format returns a deterministic, color-free debug representation of tok.
func Format(tok Token) string {
	return fmt.Sprintf("%s %s %d:%d-%d:%d", tok.Kind, strconv.Quote(tok.Text),
		tok.Span.Start.Line, tok.Span.Start.Column, tok.Span.End.Line, tok.Span.End.Column)
}
