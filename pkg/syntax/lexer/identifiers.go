package lexer

import (
	"unicode"

	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

func isIdentifierStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentifierContinue(r rune) bool {
	return isIdentifierStart(r) || unicode.IsDigit(r) || unicode.IsMark(r) || unicode.Is(unicode.Pc, r)
}

func (l *Lexer) scanIdentifier() {
	start := l.position()
	for !l.eof() {
		r, width := l.currentRune()
		if width == 1 && r == unicode.ReplacementChar || !isIdentifierContinue(r) {
			break
		}
		l.advanceRune()
	}
	text := string(l.source.Content[start.Offset:l.offset])
	kind := token.LookupKeyword(text)
	value := ""
	if kind == token.Identifier || kind == token.True || kind == token.False {
		value = text
	}
	l.emit(kind, start, value)
}
