// Package token defines Mosaic lexical tokens.
package token

import "fmt"

// Kind identifies a lexical token category.
type Kind uint16

const (
	EOF Kind = iota
	Invalid
	ByteOrderMark
	Whitespace
	Newline
	LineComment
	BlockComment
	Identifier
	String
	Integer
	Decimal
	True
	False
	Null
	LeftBrace
	RightBrace
	LeftParen
	RightParen
	LeftBracket
	RightBracket
	Colon
	Comma
	Dot
	Question
	Equal
	EqualEqual
	Bang
	BangEqual
	Less
	LessEqual
	Greater
	GreaterEqual
	AndAnd
	OrOr
	Plus
	Minus
	Star
	Slash
	Arrow
	KeywordType
	KeywordEnum
	KeywordModule
	KeywordResource
	KeywordVariant
	KeywordEnvironment
	KeywordTransform
	KeywordPolicy
	KeywordTest
	KeywordUse
	KeywordApply
	KeywordEnable
	KeywordSelect
	KeywordWhere
	KeywordResolve
	KeywordRequire
	KeywordDeny
	KeywordWarn
	KeywordExport
	KeywordProtected
	KeywordExtension
	KeywordFor
	KeywordIn
	KeywordIf
	KeywordElse
	KeywordAs
)

var kindNames = [...]string{
	"EOF", "Invalid", "ByteOrderMark", "Whitespace", "Newline", "LineComment", "BlockComment",
	"Identifier", "String", "Integer", "Decimal", "True", "False", "Null",
	"LeftBrace", "RightBrace", "LeftParen", "RightParen", "LeftBracket", "RightBracket",
	"Colon", "Comma", "Dot", "Question", "Equal", "EqualEqual", "Bang", "BangEqual",
	"Less", "LessEqual", "Greater", "GreaterEqual", "AndAnd", "OrOr", "Plus", "Minus",
	"Star", "Slash", "Arrow", "KeywordType", "KeywordEnum", "KeywordModule", "KeywordResource",
	"KeywordVariant", "KeywordEnvironment", "KeywordTransform", "KeywordPolicy", "KeywordTest",
	"KeywordUse", "KeywordApply", "KeywordEnable", "KeywordSelect", "KeywordWhere", "KeywordResolve",
	"KeywordRequire", "KeywordDeny", "KeywordWarn", "KeywordExport", "KeywordProtected",
	"KeywordExtension", "KeywordFor", "KeywordIn", "KeywordIf", "KeywordElse", "KeywordAs",
}

// String returns a stable developer-facing name for the kind.
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", k)
}
