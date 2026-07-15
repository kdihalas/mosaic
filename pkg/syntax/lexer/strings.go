package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

const supportedEscapesNote = "supported escapes are: \\\", \\\\, \\n, \\r, \\t, \\b, \\f, \\0, \\uXXXX, and \\UXXXXXXXX"

func (l *Lexer) scanString() {
	start := l.position()
	l.advanceASCII(1)
	var value strings.Builder
	valid := true

	for !l.eof() {
		if l.currentByte() == '\n' || l.currentByte() == '\r' {
			span := l.spanFrom(start)
			l.emit(token.Invalid, start, "")
			l.report("LEX003", span, "unterminated string literal; close the string before the newline")
			return
		}
		if l.currentByte() == '"' {
			l.advanceASCII(1)
			if valid {
				l.emit(token.String, start, value.String())
			} else {
				l.emit(token.Invalid, start, "")
			}
			return
		}
		if l.currentByte() == '\\' {
			if !l.scanEscape(&value) {
				valid = false
			}
			continue
		}
		if l.hasBOM() {
			badStart := l.position()
			l.offset += 3
			l.column++
			l.report("LEX009", l.spanFrom(badStart), "UTF-8 byte-order mark is only allowed at the start of a source file")
			valid = false
			continue
		}

		r, width := l.currentRune()
		if r == utf8.RuneError && width == 1 {
			badStart := l.position()
			badByte := l.currentByte()
			l.offset++
			l.column++
			l.report("LEX002", l.spanFrom(badStart), fmt.Sprintf("invalid UTF-8 encoding byte 0x%02X", badByte))
			valid = false
			continue
		}
		runeStart := l.position()
		l.advanceRune()
		if r < 0x20 {
			l.report("LEX006", l.spanFrom(runeStart), fmt.Sprintf("unescaped control character U+%04X in string literal", r), supportedEscapesNote)
			valid = false
			continue
		}
		value.Write(l.source.Content[runeStart.Offset:l.offset])
	}

	span := l.spanFrom(start)
	l.emit(token.Invalid, start, "")
	l.report("LEX003", span, "unterminated string literal; expected `\"` before end of file")
}

func (l *Lexer) scanEscape(value *strings.Builder) bool {
	start := l.position()
	l.advanceASCII(1)
	if l.eof() || l.currentByte() == '\n' || l.currentByte() == '\r' {
		l.report("LEX004", l.spanFrom(start), "incomplete string escape", supportedEscapesNote)
		return false
	}
	if l.hasBOM() {
		badStart := l.position()
		l.offset += 3
		l.column++
		l.report("LEX009", l.spanFrom(badStart), "UTF-8 byte-order mark is only allowed at the start of a source file")
		return false
	}
	if r, width := l.currentRune(); r == utf8.RuneError && width == 1 {
		badStart := l.position()
		badByte := l.currentByte()
		l.offset++
		l.column++
		l.report("LEX002", l.spanFrom(badStart), fmt.Sprintf("invalid UTF-8 encoding byte 0x%02X", badByte))
		return false
	}

	b := l.currentByte()
	switch b {
	case '"':
		value.WriteByte('"')
	case '\\':
		value.WriteByte('\\')
	case 'n':
		value.WriteByte('\n')
	case 'r':
		value.WriteByte('\r')
	case 't':
		value.WriteByte('\t')
	case 'b':
		value.WriteByte('\b')
	case 'f':
		value.WriteByte('\f')
	case '0':
		value.WriteByte(0)
	case 'u', 'U':
		digits := 4
		if b == 'U' {
			digits = 8
		}
		l.advanceASCII(1)
		return l.scanUnicodeEscape(start, digits, value)
	default:
		l.advanceRune()
		text := string(l.source.Content[start.Offset:l.offset])
		l.report("LEX004", l.spanFrom(start), fmt.Sprintf("invalid string escape %s", strconv.Quote(text)), supportedEscapesNote)
		return false
	}
	l.advanceASCII(1)
	return true
}

func (l *Lexer) scanUnicodeEscape(start diagnostics.Position, digits int, value *strings.Builder) bool {
	var scalar rune
	valid := true
	consumed := 0
	for consumed < digits && !l.eof() && l.currentByte() != '"' && l.currentByte() != '\n' && l.currentByte() != '\r' {
		b := l.currentByte()
		r, width := l.currentRune()
		if r == utf8.RuneError && width == 1 {
			badStart := l.position()
			l.offset++
			l.column++
			l.report("LEX002", l.spanFrom(badStart), fmt.Sprintf("invalid UTF-8 encoding byte 0x%02X", b))
			valid = false
			consumed++
			continue
		}
		hex, ok := hexValue(b)
		if !ok || width != 1 {
			valid = false
			l.advanceRune()
		} else {
			scalar = scalar*16 + rune(hex)
			l.advanceASCII(1)
		}
		consumed++
	}
	if consumed != digits || !valid || scalar > unicode.MaxRune || (scalar >= 0xd800 && scalar <= 0xdfff) {
		l.report("LEX005", l.spanFrom(start), "invalid Unicode escape; use exactly the required hexadecimal digits for a Unicode scalar value")
		return false
	}
	value.WriteRune(scalar)
	return true
}

func hexValue(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}
