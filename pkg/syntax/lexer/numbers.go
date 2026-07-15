package lexer

import (
	"strings"

	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

func (l *Lexer) scanNumber() {
	start := l.position()
	invalid := false
	hadIntegerSeparator := false

	componentInvalid, hadSeparator := l.scanDigitComponent()
	invalid = invalid || componentInvalid
	hadIntegerSeparator = hadSeparator
	kind := token.Integer

	if l.currentByte() == '.' && isASCIIDigit(l.peekByte(1)) {
		kind = token.Decimal
		l.advanceASCII(1)
		componentInvalid, _ = l.scanDigitComponent()
		invalid = invalid || componentInvalid
	} else if hadIntegerSeparator && l.currentByte() == '.' && l.peekByte(1) == '_' && isASCIIDigit(l.peekByte(2)) {
		// The specification classifies this separator-adjacent form as one
		// malformed numeric candidate, unlike the boundary case 1._5.
		kind = token.Decimal
		invalid = true
		l.advanceASCII(1)
		componentInvalid, _ = l.scanDigitComponent()
		invalid = invalid || componentInvalid
	}

	if l.currentByte() == 'e' || l.currentByte() == 'E' {
		kind = token.Decimal
		l.advanceASCII(1)
		if l.currentByte() == '+' || l.currentByte() == '-' {
			l.advanceASCII(1)
		}
		if !isASCIIDigit(l.currentByte()) {
			invalid = true
			for isASCIIDigit(l.currentByte()) || l.currentByte() == '_' {
				l.advanceASCII(1)
			}
		} else {
			componentInvalid, _ = l.scanDigitComponent()
			invalid = invalid || componentInvalid
		}
	}

	text := string(l.source.Content[start.Offset:l.offset])
	if invalid {
		span := l.spanFrom(start)
		l.emit(token.Invalid, start, "")
		l.report("LEX008", span, "malformed numeric literal; underscores must separate digits and exponents require digits")
		return
	}
	l.emit(kind, start, strings.ReplaceAll(text, "_", ""))
}

func (l *Lexer) scanDigitComponent() (invalid, hadSeparator bool) {
	previousDigit := false
	for !l.eof() {
		b := l.currentByte()
		switch {
		case isASCIIDigit(b):
			previousDigit = true
			l.advanceASCII(1)
		case b == '_':
			hadSeparator = true
			if !previousDigit || !isASCIIDigit(l.peekByte(1)) {
				invalid = true
			}
			previousDigit = false
			l.advanceASCII(1)
		default:
			return invalid, hadSeparator
		}
	}
	return invalid, hadSeparator
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }
