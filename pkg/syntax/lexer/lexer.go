// Package lexer converts Mosaic source bytes into a lossless token stream.
package lexer

import (
	"fmt"
	"unicode/utf8"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

// Options controls lexing behavior.
type Options struct {
	// MaxDiagnostics limits ordinary diagnostics. Non-positive values are unlimited.
	MaxDiagnostics int
}

// Result is the lossless token stream and ordered source diagnostics.
type Result struct {
	Tokens      []token.Token
	Diagnostics diagnostics.List
}

// Lex tokenizes src deterministically. It always returns exactly one final EOF token.
func Lex(src source.File, options Options) Result {
	l := Lexer{
		source:         src,
		line:           1,
		column:         1,
		maxDiagnostics: options.MaxDiagnostics,
	}
	l.run()
	return Result{Tokens: l.tokens, Diagnostics: l.diagnostics}
}

// Lexer holds the state for one lexical pass.
type Lexer struct {
	source source.File
	offset int
	line   int
	column int

	tokens      []token.Token
	diagnostics diagnostics.List

	maxDiagnostics      int
	diagnosticCount     int
	diagnosticLimitSent bool
}

func (l *Lexer) run() {
	if l.hasBOM() {
		start := l.position()
		l.offset += len(utf8BOM) // An initial BOM has no visible column width.
		l.emit(token.ByteOrderMark, start, "")
	}

	for !l.eof() {
		before := l.offset
		l.scanToken()
		if l.offset <= before {
			panic("lexer made no progress")
		}
	}

	position := l.position()
	l.tokens = append(l.tokens, token.Token{
		Kind: token.EOF,
		Span: diagnostics.Span{SourceName: l.source.Name, Start: position, End: position},
	})
}

func (l *Lexer) scanToken() {
	if l.hasBOM() {
		l.scanMisplacedBOM()
		return
	}

	b := l.currentByte()
	switch {
	case b == ' ' || b == '\t' || b == '\v' || b == '\f':
		l.scanWhitespace()
	case b == '\n' || b == '\r':
		l.scanNewline()
	case b == '"':
		l.scanString()
	case b >= '0' && b <= '9':
		l.scanNumber()
	case b == '/' && l.peekByte(1) == '/':
		l.scanLineComment()
	case b == '/' && l.peekByte(1) == '*':
		l.scanBlockComment()
	default:
		r, width := l.currentRune()
		if r == utf8.RuneError && width == 1 {
			l.scanInvalidUTF8()
			return
		}
		if isIdentifierStart(r) {
			l.scanIdentifier()
			return
		}
		if l.scanOperator() {
			return
		}
		start := l.position()
		l.advanceRune()
		span := l.spanFrom(start)
		l.emit(token.Invalid, start, "")
		l.report("LEX001", span, fmt.Sprintf("unexpected character %q", r))
	}
}

func (l *Lexer) eof() bool { return l.offset >= len(l.source.Content) }

func (l *Lexer) currentByte() byte {
	if l.eof() {
		return 0
	}
	return l.source.Content[l.offset]
}

func (l *Lexer) peekByte(distance int) byte {
	index := l.offset + distance
	if index < 0 || index >= len(l.source.Content) {
		return 0
	}
	return l.source.Content[index]
}

func (l *Lexer) currentRune() (rune, int) {
	if l.eof() {
		return 0, 0
	}
	return utf8.DecodeRune(l.source.Content[l.offset:])
}

func (l *Lexer) advanceRune() (rune, int) {
	r, width := l.currentRune()
	if width == 0 {
		return r, width
	}
	l.offset += width
	l.column++
	return r, width
}

func (l *Lexer) advanceASCII(count int) {
	l.offset += count
	l.column += count
}

func (l *Lexer) advanceNewline() {
	if l.currentByte() == '\r' && l.peekByte(1) == '\n' {
		l.offset += 2
	} else {
		l.offset++
	}
	l.line++
	l.column = 1
}

func (l *Lexer) position() diagnostics.Position {
	return diagnostics.Position{Offset: l.offset, Line: l.line, Column: l.column}
}

func (l *Lexer) spanFrom(start diagnostics.Position) diagnostics.Span {
	return diagnostics.Span{SourceName: l.source.Name, Start: start, End: l.position()}
}

func (l *Lexer) emit(kind token.Kind, start diagnostics.Position, value string) {
	text := string(l.source.Content[start.Offset:l.offset])
	l.tokens = append(l.tokens, token.Token{Kind: kind, Text: text, Value: value, Span: l.spanFrom(start)})
}

func (l *Lexer) report(code string, span diagnostics.Span, message string, notes ...string) {
	if l.diagnosticLimitSent {
		return
	}
	l.diagnostics = append(l.diagnostics, diagnostics.Diagnostic{
		Code: code, Severity: diagnostics.SeverityError, Message: message, Span: span, Notes: notes,
	})
	l.diagnosticCount++
	if l.maxDiagnostics > 0 && l.diagnosticCount >= l.maxDiagnostics {
		position := span.End
		limitSpan := diagnostics.Span{SourceName: l.source.Name, Start: position, End: position}
		l.diagnostics = append(l.diagnostics, diagnostics.Diagnostic{
			Code: "LEX010", Severity: diagnostics.SeverityError,
			Message: "diagnostic limit reached; further lexer diagnostics were suppressed", Span: limitSpan,
		})
		l.diagnosticLimitSent = true
	}
}

func (l *Lexer) hasBOM() bool {
	return l.offset+3 <= len(l.source.Content) &&
		l.source.Content[l.offset] == utf8BOM[0] &&
		l.source.Content[l.offset+1] == utf8BOM[1] &&
		l.source.Content[l.offset+2] == utf8BOM[2]
}

func (l *Lexer) scanMisplacedBOM() {
	start := l.position()
	l.offset += 3
	l.column++
	span := l.spanFrom(start)
	l.emit(token.Invalid, start, "")
	l.report("LEX009", span, "UTF-8 byte-order mark is only allowed at the start of a source file")
}

func (l *Lexer) scanInvalidUTF8() {
	start := l.position()
	value := l.currentByte()
	l.offset++
	l.column++
	span := l.spanFrom(start)
	l.emit(token.Invalid, start, "")
	l.report("LEX002", span, fmt.Sprintf("invalid UTF-8 encoding byte 0x%02X", value))
}

func (l *Lexer) scanWhitespace() {
	start := l.position()
	for !l.eof() {
		b := l.currentByte()
		if b != ' ' && b != '\t' && b != '\v' && b != '\f' {
			break
		}
		l.advanceASCII(1)
	}
	l.emit(token.Whitespace, start, "")
}

func (l *Lexer) scanNewline() {
	start := l.position()
	l.advanceNewline()
	l.emit(token.Newline, start, "")
}
