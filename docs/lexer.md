# Mosaic lexer

The public lexer API accepts an in-memory `source.File` and returns a lossless
token stream plus structured diagnostics:

```go
result := lexer.Lex(source.NewFile("app.mosaic", content), lexer.Options{})
```

The lexer never reads files or environment state. It always preserves trivia
and appends exactly one EOF token. Concatenating `Token.Text` for every token
except EOF reconstructs the original bytes, including invalid UTF-8.

## Tokens and words

Structural tokens are `EOF` and `Invalid`. Trivia consists of
`ByteOrderMark`, `Whitespace`, `Newline`, `LineComment`, and `BlockComment`.
Literals are `Identifier`, `String`, `Integer`, `Decimal`, `True`, `False`, and
`Null`. The remaining tokens are delimiters, separators, operators, and the
hard keywords listed in the lexical grammar.

Hard keyword matching is exact and case-sensitive. Domain words such as
`input`, `workload`, `service`, `set`, `add`, and `target` are contextual and
remain identifiers. Identifiers support Unicode letters, decimal digits,
marks, connector punctuation, and `_`; they are not Unicode-normalized.

## Positions and trivia

Offsets are zero-based byte offsets. Lines and columns are one-based and spans
have exclusive ends. Columns count decoded Unicode code points; tabs and
invalid bytes each advance by one. LF, CRLF, and CR each count as one newline.
An initial UTF-8 BOM is trivia and advances the byte offset without advancing
the visible column. A BOM elsewhere is invalid.

Spaces, tabs, vertical tabs, and form feeds are grouped as whitespace. Each
physical newline is a separate token with its original spelling. Line comments
exclude their newline. Block comments may nest and are emitted as one token.

## Literals

Numbers are scanned with a state machine. Decimal integers may contain `_`
only between digits. Decimals have either a fractional part whose dot is
followed by a digit, or an exponent with at least one digit. Signs belong only
to exponents; a leading minus is its own token. Malformed candidates such as
`1e+`, `1__0`, and `1_000._5` are a single invalid token. `1.`, `.5`,
`1._5`, and `123abc` split at their lexical boundaries.

Strings use double quotes and support `\"`, `\\`, `\n`, `\r`, `\t`, `\b`,
`\f`, `\0`, `\uXXXX`, and `\UXXXXXXXX`. Unicode escapes must encode scalar
values, not surrogates. Raw newlines terminate an invalid string before the
newline. Other raw controls, invalid escapes, and invalid UTF-8 make the whole
string token invalid while scanning continues to the closing quote.

## Recovery and diagnostics

The stable diagnostics are LEX001 through LEX010: unexpected character,
invalid UTF-8, unterminated string, invalid escape, invalid Unicode escape,
unescaped string control, unterminated block comment, malformed number,
misplaced BOM, and diagnostic limit reached. Invalid input always consumes
bytes. Invalid UTF-8 within an atomic string/comment is diagnosed byte by byte;
strings become invalid while comments retain their complete comment token.

With a positive diagnostic limit, the lexer emits that many ordinary
diagnostics, immediately adds one LEX010 at the end of the final permitted
diagnostic, then suppresses later diagnostics while continuing to tokenize.
Non-positive limits are unlimited.

## Initial test matrix

Coverage includes all kinds and classifications; hard and contextual words;
ASCII and Unicode identifiers; LF, CRLF, CR, tabs, BOM, and EOF positions; all
operators; nested and unterminated comments; valid and malformed numeric
states; every string escape and failure mode; unexpected runes and invalid
bytes; recovery, diagnostic limits, golden Mosaic programs, byte round trips,
fuzz invariants, determinism, and representative benchmarks.

## MVP limitations

The lexer intentionally has no interpolation, raw or multiline strings,
heredocs, character literals, non-decimal numbers, numeric units, unquoted
durations, semicolon insertion, significant indentation, preprocessing,
macros, conditional or incremental lexing, or parser-level recovery.
