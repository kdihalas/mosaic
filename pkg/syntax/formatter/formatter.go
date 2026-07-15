// Package formatter provides deterministic source formatting over parsed files.
package formatter

import (
	"bytes"
	"strings"

	"github.com/kdihalas/mosaic/pkg/syntax/ast"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

// Format regenerates whitespace while preserving exact non-whitespace tokens,
// including comments and quoted keys. The file must have parsed successfully.
func Format(f *ast.File) []byte {
	var out bytes.Buffer
	indent := 0
	line := make([]token.Token, 0, 16)
	blank := false
	flush := func() {
		if len(line) == 0 {
			if !blank && out.Len() > 0 {
				out.WriteByte('\n')
				blank = true
			}
			return
		}
		first := firstCode(line)
		if first == token.RightBrace && indent > 0 {
			indent--
		}
		out.WriteString(strings.Repeat("    ", indent))
		writeLine(&out, line)
		out.WriteByte('\n')
		blank = false
		if lastCode(line) == token.LeftBrace {
			indent++
		}
		line = line[:0]
	}
	for _, t := range f.Tokens {
		switch t.Kind {
		case token.EOF:
			flush()
		case token.Whitespace, token.ByteOrderMark:
		case token.Newline:
			flush()
		default:
			line = append(line, t)
		}
	}
	return bytes.TrimRight(out.Bytes(), "\n ")
}
func firstCode(ts []token.Token) token.Kind {
	for _, t := range ts {
		if t.Kind != token.LineComment && t.Kind != token.BlockComment {
			return t.Kind
		}
	}
	return token.Invalid
}
func lastCode(ts []token.Token) token.Kind {
	for i := len(ts) - 1; i >= 0; i-- {
		if ts[i].Kind != token.LineComment && ts[i].Kind != token.BlockComment {
			return ts[i].Kind
		}
	}
	return token.Invalid
}
func writeLine(b *bytes.Buffer, ts []token.Token) {
	for i, t := range ts {
		if i > 0 && space(ts[i-1], t) {
			b.WriteByte(' ')
		}
		b.WriteString(t.Text)
	}
}
func space(a, b token.Token) bool {
	if a.Kind == token.LineComment {
		return false
	}
	if b.Kind == token.LineComment || b.Kind == token.BlockComment {
		return true
	}
	if b.Kind == token.Comma || b.Kind == token.Dot || b.Kind == token.RightParen || b.Kind == token.RightBracket {
		return false
	}
	if a.Kind == token.Dot || a.Kind == token.LeftParen || a.Kind == token.LeftBracket {
		return false
	}
	if b.Kind == token.LeftParen {
		return a.Kind.IsKeyword()
	}
	if b.Kind == token.LeftBrace {
		return true
	}
	if a.Kind == token.Comma {
		return true
	}
	return true
}
