# Mosaic grammar

The lexer specification is documented in [lexer.md](lexer.md). The parser uses
that token stream directly. The core syntactic grammar is:

```ebnf
file = { declaration } ;
declaration = typeDecl | enumDecl | moduleDecl | useDecl | variantDecl
            | environmentDecl | transformDecl | policyDecl | testDecl ;
typeDecl = "type", identifier, "{", { field | requireStmt }, "}" ;
field = identifier, ":", expression, [ "=", expression ] ;
enumDecl = "enum", identifier, "{", { identifier, [","] }, "}" ;
moduleDecl = "module", identifier, "(", identifier, ":", expression, ")", body ;
useDecl = "use", identifier, "as", identifier, body ;
variantDecl = "variant", identifier, body ;
environmentDecl = "environment", identifier, body ;
transformDecl = "transform", identifier, body ;
policyDecl = "policy", identifier, body ;
testDecl = "test", identifier, body ;
body = "{", { statement, [","] }, "}" ;
expression = primary, { postfix | binaryOperator, expression } ;
```

Expression precedence, high to low, is primary/postfix, unary, multiplication,
addition, comparison, equality, logical AND, logical OR.

## Lexical reference

```ebnf
identifierStart    = "_" | UnicodeLetter ;
identifierContinue = identifierStart | UnicodeDigit | UnicodeMark
                   | UnicodeConnectorPunctuation ;
identifier         = identifierStart, { identifierContinue } ;

digit              = "0" ... "9" ;
digits             = digit, { digit | "_", digit } ;
integer            = digits ;
exponent           = ("e" | "E"), ["+" | "-"], digits ;
decimal            = digits, ".", digits, [exponent]
                   | digits, exponent ;

string             = '"', { stringCharacter | escape }, '"' ;
escape             = '\\', ('"' | "\\" | "n" | "r" | "t" | "b" | "f" | "0")
                   | "\\u", hexDigit, hexDigit, hexDigit, hexDigit
                   | "\\U", hexDigit, hexDigit, hexDigit, hexDigit,
                              hexDigit, hexDigit, hexDigit, hexDigit ;

lineComment        = "//", { nonNewlineCharacter } ;
blockComment       = "/*", blockCommentContent, "*/" ;
```

Block comments nest using a depth counter. String characters exclude raw
newlines, unescaped code points below U+0020, quote, and backslash.

Hard keywords are `type`, `enum`, `module`, `resource`, `variant`,
`environment`, `transform`, `policy`, `test`, `use`, `apply`, `enable`,
`select`, `where`, `resolve`, `require`, `deny`, `warn`, `export`, `protected`,
`extension`, `for`, `in`, `if`, `else`, and `as`. The words `true`, `false`,
and `null` have literal kinds. All matching is exact and case-sensitive.

The two-character operators `->`, `==`, `!=`, `<=`, `>=`, `&&`, and `||` are
matched before their one-character prefixes. Comment openers are matched before
the slash operator.
