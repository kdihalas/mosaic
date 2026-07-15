package lexer

import "github.com/kdihalas/mosaic/pkg/syntax/token"

func (l *Lexer) scanOperator() bool {
	start := l.position()
	two := string([]byte{l.currentByte(), l.peekByte(1)})
	if kind, ok := mapTwoCharacterOperator(two); ok {
		l.advanceASCII(2)
		l.emit(kind, start, "")
		return true
	}

	var kind token.Kind
	switch l.currentByte() {
	case '{':
		kind = token.LeftBrace
	case '}':
		kind = token.RightBrace
	case '(':
		kind = token.LeftParen
	case ')':
		kind = token.RightParen
	case '[':
		kind = token.LeftBracket
	case ']':
		kind = token.RightBracket
	case ':':
		kind = token.Colon
	case ',':
		kind = token.Comma
	case '.':
		kind = token.Dot
	case '?':
		kind = token.Question
	case '=':
		kind = token.Equal
	case '!':
		kind = token.Bang
	case '<':
		kind = token.Less
	case '>':
		kind = token.Greater
	case '+':
		kind = token.Plus
	case '-':
		kind = token.Minus
	case '*':
		kind = token.Star
	case '/':
		kind = token.Slash
	default:
		return false
	}
	l.advanceASCII(1)
	l.emit(kind, start, "")
	return true
}

func mapTwoCharacterOperator(text string) (token.Kind, bool) {
	switch text {
	case "->":
		return token.Arrow, true
	case "==":
		return token.EqualEqual, true
	case "!=":
		return token.BangEqual, true
	case "<=":
		return token.LessEqual, true
	case ">=":
		return token.GreaterEqual, true
	case "&&":
		return token.AndAnd, true
	case "||":
		return token.OrOr, true
	default:
		return 0, false
	}
}
