package lexer

import (
	"fmt"
	"unicode/utf8"

	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

func (l *Lexer) scanLineComment() {
	start := l.position()
	l.advanceASCII(2)
	for !l.eof() && l.currentByte() != '\n' && l.currentByte() != '\r' {
		l.scanCommentRune()
	}
	l.emit(token.LineComment, start, "")
}

func (l *Lexer) scanBlockComment() {
	start := l.position()
	l.advanceASCII(2)
	depth := 1
	for !l.eof() {
		if l.currentByte() == '/' && l.peekByte(1) == '*' {
			depth++
			l.advanceASCII(2)
			continue
		}
		if l.currentByte() == '*' && l.peekByte(1) == '/' {
			depth--
			l.advanceASCII(2)
			if depth == 0 {
				l.emit(token.BlockComment, start, "")
				return
			}
			continue
		}
		if l.currentByte() == '\n' || l.currentByte() == '\r' {
			l.advanceNewline()
			continue
		}
		l.scanCommentRune()
	}

	span := l.spanFrom(start)
	l.emit(token.Invalid, start, "")
	l.report("LEX007", span, "unterminated block comment; expected `*/` before end of file")
}

func (l *Lexer) scanCommentRune() {
	if l.hasBOM() {
		start := l.position()
		l.offset += 3
		l.column++
		l.report("LEX009", l.spanFrom(start), "UTF-8 byte-order mark is only allowed at the start of a source file")
		return
	}
	r, width := l.currentRune()
	if r == utf8.RuneError && width == 1 {
		start := l.position()
		value := l.currentByte()
		l.offset++
		l.column++
		l.report("LEX002", l.spanFrom(start), fmt.Sprintf("invalid UTF-8 encoding byte 0x%02X", value))
		return
	}
	l.advanceRune()
}
