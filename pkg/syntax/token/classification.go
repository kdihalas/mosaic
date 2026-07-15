package token

// IsTrivia reports whether k represents preserved lexical trivia.
func (k Kind) IsTrivia() bool {
	return k >= ByteOrderMark && k <= BlockComment
}

// IsKeyword reports whether k is a hard keyword.
func (k Kind) IsKeyword() bool {
	return k >= KeywordType && k <= KeywordAs
}

// IsLiteral reports whether k is a literal token kind.
func (k Kind) IsLiteral() bool {
	return k >= String && k <= Null
}

// IsOperator reports whether k is an operator token kind.
func (k Kind) IsOperator() bool {
	return k >= Equal && k <= Arrow
}

// IsDelimiter reports whether k is a delimiter or separator token kind.
func (k Kind) IsDelimiter() bool {
	return k >= LeftBrace && k <= Question
}
