// Package parser turns the existing lexer token stream into a Mosaic AST.
package parser

import (
	"fmt"
	"strings"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/syntax/ast"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

type Options struct{ MaxDiagnostics, MaxParseDepth, MaxExpressionDepth int }
type Result struct {
	File        *ast.File
	Diagnostics diagnostics.List
}

func Parse(src source.File, tokens []token.Token, options Options) Result {
	if options.MaxParseDepth <= 0 {
		options.MaxParseDepth = 256
	}
	if options.MaxExpressionDepth <= 0 {
		options.MaxExpressionDepth = 256
	}
	p := &parser{src: src, tokens: token.Significant(tokens), options: options}
	f := &ast.File{Name: src.Name, Tokens: append([]token.Token(nil), tokens...)}
	for !p.at(token.EOF) {
		before := p.pos
		if d := p.declaration(); d != nil {
			f.Declarations = append(f.Declarations, d)
		} else {
			p.syncDeclaration()
		}
		if p.pos == before {
			p.advance()
		}
	}
	return Result{File: f, Diagnostics: p.diags}
}

type parser struct {
	src                          source.File
	tokens                       []token.Token
	pos, depth, exprDepth, count int
	options                      Options
	diags                        diagnostics.List
	limited                      bool
}

func (p *parser) cur() token.Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}
func (p *parser) peek(n int) token.Token {
	i := p.pos + n
	if i >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[i]
}
func (p *parser) at(k token.Kind) bool { return p.cur().Kind == k }
func (p *parser) advance() token.Token {
	t := p.cur()
	if !p.at(token.EOF) {
		p.pos++
	}
	return t
}
func (p *parser) match(k token.Kind) bool {
	if p.at(k) {
		p.advance()
		return true
	}
	return false
}
func (p *parser) report(code, msg string, s diagnostics.Span) {
	if p.limited {
		return
	}
	p.diags = append(p.diags, diagnostics.Diagnostic{Code: code, Severity: diagnostics.SeverityError, Message: msg, Span: s})
	p.count++
	if p.options.MaxDiagnostics > 0 && p.count >= p.options.MaxDiagnostics {
		p.diags = append(p.diags, diagnostics.Diagnostic{Code: "PAR010", Severity: diagnostics.SeverityError, Message: "parser diagnostic limit reached", Span: s})
		p.limited = true
	}
}
func (p *parser) expect(k token.Kind, text string) token.Token {
	if p.at(k) {
		return p.advance()
	}
	t := p.cur()
	p.report("PAR004", "expected "+text, t.Span)
	return t
}
func span(a, b diagnostics.Span) diagnostics.Span {
	return diagnostics.Span{SourceName: a.SourceName, Start: a.Start, End: b.End}
}
func base(a, b diagnostics.Span) ast.Base { return ast.Base{SourceSpan: span(a, b)} }

func (p *parser) declaration() ast.Declaration {
	start := p.cur()
	switch start.Kind {
	case token.KeywordType:
		return p.typeDecl()
	case token.KeywordEnum:
		return p.enumDecl()
	case token.KeywordModule:
		return p.moduleDecl()
	case token.KeywordUse:
		return p.useDecl()
	case token.KeywordVariant, token.KeywordEnvironment, token.KeywordTransform, token.KeywordPolicy, token.KeywordTest:
		p.advance()
		name := p.ident()
		body, end := p.body()
		b := base(start.Span, end)
		switch start.Kind {
		case token.KeywordVariant:
			return &ast.VariantDeclaration{Base: b, Name: name, Body: body}
		case token.KeywordEnvironment:
			return &ast.EnvironmentDeclaration{Base: b, Name: name, Body: body}
		case token.KeywordTransform:
			return &ast.TransformDeclaration{Base: b, Name: name, Body: body}
		case token.KeywordPolicy:
			return &ast.PolicyDeclaration{Base: b, Name: name, Body: body}
		default:
			return &ast.TestDeclaration{Base: b, Name: name, Body: body}
		}
	default:
		p.report("PAR001", "expected declaration", start.Span)
		return nil
	}
}
func (p *parser) ident() string {
	if p.at(token.Identifier) || p.cur().Kind.IsKeyword() {
		return p.advance().Text
	}
	p.report("PAR002", "expected identifier", p.cur().Span)
	return ""
}
func (p *parser) typeDecl() ast.Declaration {
	start := p.advance()
	name := p.ident()
	p.expect(token.LeftBrace, "`{`")
	var fields []ast.TypeField
	var stmts []ast.Statement
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		before := p.pos
		if p.at(token.KeywordRequire) {
			stmts = append(stmts, p.rule("require"))
		} else if p.at(token.Identifier) && p.peek(1).Kind == token.Colon {
			n := p.advance()
			p.advance()
			typ := p.typeExpression()
			var def ast.Expression
			if p.match(token.Equal) {
				def = p.expression(0)
			}
			e := n.Span
			if def != nil {
				e = def.Span()
			} else if typ != nil {
				e = typ.Span()
			}
			fields = append(fields, ast.TypeField{Base: base(n.Span, e), Name: n.Text, Type: typ, Default: def})
		} else {
			p.report("PAR008", "invalid type declaration body", p.cur().Span)
			p.syncStatement()
		}
		if before == p.pos {
			p.advance()
		}
		p.match(token.Comma)
	}
	end := p.expect(token.RightBrace, "`}` to close type declaration").Span
	return &ast.TypeDeclaration{Base: base(start.Span, end), Name: name, Fields: fields, Body: stmts}
}
func (p *parser) enumDecl() ast.Declaration {
	start := p.advance()
	name := p.ident()
	p.expect(token.LeftBrace, "`{`")
	var ms []ast.IdentifierExpression
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		t := p.expect(token.Identifier, "enum member")
		ms = append(ms, ast.IdentifierExpression{Base: ast.Base{SourceSpan: t.Span}, Name: t.Text})
		p.match(token.Comma)
	}
	end := p.expect(token.RightBrace, "`}` to close enum").Span
	return &ast.EnumDeclaration{Base: base(start.Span, end), Name: name, Members: ms}
}
func (p *parser) moduleDecl() ast.Declaration {
	start := p.advance()
	name := p.ident()
	var param *ast.Parameter
	if p.match(token.LeftParen) {
		s := p.cur()
		n := p.ident()
		p.expect(token.Colon, "`:`")
		t := p.typeExpression()
		e := p.expect(token.RightParen, "`)`").Span
		param = &ast.Parameter{Base: base(s.Span, e), Name: n, Type: t}
	}
	body, end := p.body()
	return &ast.ModuleDeclaration{Base: base(start.Span, end), Name: name, Parameter: param, Body: body}
}

func (p *parser) typeExpression() ast.Expression {
	start := p.cur()
	if start.Kind != token.Identifier && !start.Kind.IsKeyword() {
		p.report("PAR003", "expected type expression", start.Span)
		return nil
	}
	p.advance()
	var out ast.Expression = &ast.IdentifierExpression{Base: ast.Base{SourceSpan: start.Span}, Name: start.Text}
	if p.match(token.Less) {
		var args []ast.Expression
		for !p.at(token.Greater) && !p.at(token.EOF) {
			x := p.typeExpression()
			if x == nil {
				break
			}
			args = append(args, x)
			if !p.match(token.Comma) {
				break
			}
		}
		end := p.expect(token.Greater, "`>` to close type arguments").Span
		out = &ast.CallExpression{Base: base(start.Span, end), Callee: out, Arguments: args}
	}
	return out
}
func (p *parser) useDecl() ast.Declaration {
	start := p.advance()
	mod := p.ident()
	p.expect(token.KeywordAs, "`as`")
	alias := p.ident()
	body, end := p.body()
	return &ast.ModuleUseDeclaration{Base: base(start.Span, end), Module: mod, Alias: alias, Body: body}
}
func (p *parser) body() ([]ast.Statement, diagnostics.Span) {
	open := p.expect(token.LeftBrace, "`{`")
	p.depth++
	if p.depth > p.options.MaxParseDepth {
		p.report("PAR005", "maximum parse depth exceeded", open.Span)
	}
	var out []ast.Statement
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		before := p.pos
		if s := p.statement(); s != nil {
			out = append(out, s)
		} else {
			p.syncStatement()
		}
		if before == p.pos {
			p.advance()
		}
		p.match(token.Comma)
	}
	end := p.cur().Span
	if p.at(token.RightBrace) {
		end = p.advance().Span
	} else {
		p.report("PAR006", "expected closing `}`", open.Span)
	}
	p.depth--
	return out, end
}

func (p *parser) statement() ast.Statement {
	t := p.cur()
	switch t.Kind {
	case token.KeywordUse:
		p.advance()
		n := p.ident()
		return &ast.UseStatement{Base: base(t.Span, p.tokens[p.pos-1].Span), Name: n}
	case token.KeywordApply:
		p.advance()
		n := p.ident()
		return &ast.ApplyStatement{Base: base(t.Span, p.tokens[p.pos-1].Span), Name: n}
	case token.KeywordExport:
		p.advance()
		e := p.expression(0)
		return &ast.ExportStatement{Base: base(t.Span, e.Span()), Target: e}
	case token.KeywordExtension:
		p.advance()
		e := p.expression(0)
		return &ast.ExtensionStatement{Base: base(t.Span, e.Span()), Target: e}
	case token.KeywordProtected:
		p.advance()
		e := p.expression(0)
		return &ast.ProtectedStatement{Base: base(t.Span, e.Span()), Target: e}
	case token.KeywordRequire:
		return p.rule("require")
	case token.KeywordDeny:
		return p.rule("deny")
	case token.KeywordWarn:
		return p.rule("warn")
	case token.KeywordSelect:
		return p.selectStmt()
	case token.KeywordResource:
		p.advance()
		name := p.expect(token.String, "custom resource local name")
		body, end := p.body()
		return &ast.ResourceDeclaration{Base: base(t.Span, end), Kind: "resource", Name: name.Value, Body: body}
	case token.KeywordEnable:
		p.advance()
		return p.operation(t, "enable")
	case token.KeywordResolve:
		p.advance()
		return p.operation(t, "resolve")
	}
	if t.Kind == token.Identifier {
		switch t.Text {
		case "set", "replace", "delete", "append", "merge", "add", "assert", "build", "expect":
			p.advance()
			return p.operation(t, t.Text)
		}
		// contextual resource declaration: kind "name" { ... }
		if p.peek(1).Kind == token.String && p.peek(2).Kind == token.LeftBrace {
			kind := p.advance()
			name := p.advance()
			body, end := p.body()
			return &ast.ResourceDeclaration{Base: base(kind.Span, end), Kind: kind.Text, Name: name.Value, Body: body}
		}
		// named block input.
		if p.peek(1).Kind == token.LeftBrace {
			n := p.advance()
			body, end := p.body()
			return &ast.BlockDeclaration{Base: base(n.Span, end), Name: n.Text, Body: body}
		}
		if p.peek(1).Kind == token.Identifier && p.peek(2).Kind == token.LeftBrace {
			n := p.advance()
			label := p.advance()
			body, end := p.body()
			return &ast.BlockDeclaration{Base: base(n.Span, end), Name: n.Text, Label: label.Text, Body: body}
		}
	}
	e := p.expression(0)
	if e == nil {
		return nil
	}
	if p.match(token.Equal) {
		v := p.expression(0)
		if v == nil {
			p.report("PAR003", "expected expression", p.cur().Span)
			return nil
		}
		return &ast.AssignmentStatement{Base: base(e.Span(), v.Span()), Target: e, Value: v}
	}
	return &ast.ExpressionStatement{Base: ast.Base{SourceSpan: e.Span()}, Expression: e}
}
func (p *parser) rule(kind string) ast.Statement {
	start := p.advance()
	cond := p.expression(0)
	var body []ast.Statement
	end := cond.Span()
	if p.at(token.LeftBrace) {
		body, end = p.body()
	}
	r := ast.RequireStatement{Base: base(start.Span, end), Condition: cond, Body: body}
	switch kind {
	case "deny":
		x := ast.DenyStatement(r)
		return &x
	case "warn":
		x := ast.WarnStatement(r)
		return &x
	default:
		return &r
	}
}
func (p *parser) selectStmt() ast.Statement {
	s := p.advance()
	typ := p.ident()
	var where ast.Expression
	if p.match(token.KeywordWhere) {
		where = p.expression(0)
	}
	body, end := p.body()
	return &ast.SelectStatement{Base: base(s.Span, end), Type: typ, Where: where, Body: body}
}
func (p *parser) operation(start token.Token, op string) ast.Statement {
	x := ast.OperationStatement{Operation: op}
	var end = start.Span
	if op == "enable" {
		x.Name = p.ident()
		if p.at(token.LeftBrace) {
			x.Body, end = p.body()
		}
	}
	if op == "add" {
		x.Name = p.ident()
		if p.at(token.String) {
			x.Identity = p.advance().Value
		}
		if p.at(token.Identifier) && p.cur().Text == "to" {
			p.advance()
		}
		x.Target = p.expression(0)
		if p.at(token.LeftBrace) {
			x.Body, end = p.body()
		}
	}
	if op == "build" {
		x.Name = p.ident()
		if p.at(token.Identifier) && p.cur().Text == "with" {
			p.advance()
			x.Body, end = p.body()
		}
	}
	if op == "expect" {
		x.Name = p.ident()
		if p.at(token.Identifier) {
			x.Identity = p.advance().Text
		}
		if p.at(token.LeftBrace) {
			x.Body, end = p.body()
		}
	}
	if op == "assert" {
		x.Value = p.expression(0)
		if x.Value != nil {
			end = x.Value.Span()
		}
	}
	if op == "set" || op == "replace" || op == "resolve" {
		x.Target = p.expression(0)
		p.expect(token.Equal, "`=`")
		x.Value = p.expression(0)
		if x.Value != nil {
			end = x.Value.Span()
		}
	}
	if op == "delete" {
		x.Target = p.expression(0)
		if x.Target != nil {
			end = x.Target.Span()
		}
	}
	if op == "append" {
		x.Target = p.pathMembers()
		x.Value = p.expression(0)
		if x.Value != nil {
			end = x.Value.Span()
		}
	}
	if op == "merge" {
		x.Target = p.expression(0)
		if p.at(token.Identifier) && p.cur().Text == "by" {
			p.advance()
			x.Identity = p.ident()
		}
		if p.at(token.LeftBrace) {
			x.Body, end = p.body()
		}
	}
	x.Base = base(start.Span, end)
	switch op {
	case "enable":
		v := ast.EnableStatement(x)
		return &v
	case "set":
		v := ast.SetStatement(x)
		return &v
	case "replace":
		v := ast.ReplaceStatement(x)
		return &v
	case "delete":
		v := ast.DeleteStatement(x)
		return &v
	case "append":
		v := ast.AppendStatement(x)
		return &v
	case "merge":
		v := ast.MergeStatement(x)
		return &v
	case "add":
		v := ast.AddStatement(x)
		return &v
	case "resolve":
		v := ast.ResolveStatement(x)
		return &v
	default:
		return &x
	}
}

func (p *parser) pathMembers() ast.Expression {
	t := p.cur()
	if t.Kind != token.Identifier && !t.Kind.IsKeyword() {
		p.report("PAR003", "expected field path", t.Span)
		return nil
	}
	p.advance()
	var e ast.Expression = &ast.IdentifierExpression{Base: ast.Base{SourceSpan: t.Span}, Name: t.Text}
	for p.match(token.Dot) {
		m := p.cur()
		if m.Kind != token.Identifier && !m.Kind.IsKeyword() {
			p.report("PAR002", "expected path member", m.Span)
			break
		}
		p.advance()
		e = &ast.MemberExpression{Base: base(e.Span(), m.Span), Object: e, Member: m.Text}
	}
	return e
}

var prec = map[token.Kind]int{token.OrOr: 1, token.AndAnd: 2, token.EqualEqual: 3, token.BangEqual: 3, token.Less: 4, token.LessEqual: 4, token.Greater: 4, token.GreaterEqual: 4, token.Plus: 5, token.Minus: 5, token.Star: 6, token.Slash: 6}

func (p *parser) expression(min int) ast.Expression {
	p.exprDepth++
	defer func() { p.exprDepth-- }()
	if p.exprDepth > p.options.MaxExpressionDepth {
		p.report("PAR003", "maximum expression depth exceeded", p.cur().Span)
		return nil
	}
	left := p.prefix()
	if left == nil {
		return nil
	}
	for {
		k := p.cur().Kind
		pr, ok := prec[k]
		if !ok || pr < min {
			break
		}
		op := p.advance()
		right := p.expression(pr + 1)
		if right == nil {
			break
		}
		left = &ast.BinaryExpression{Base: base(left.Span(), right.Span()), Left: left, Operator: op.Text, Right: right}
	}
	return left
}
func (p *parser) prefix() ast.Expression {
	t := p.cur()
	var e ast.Expression
	switch t.Kind {
	case token.Identifier:
		p.advance()
		e = &ast.IdentifierExpression{Base: ast.Base{SourceSpan: t.Span}, Name: t.Text}
	case token.String:
		p.advance()
		e = &ast.StringLiteral{Base: ast.Base{SourceSpan: t.Span}, Value: t.Value, Raw: t.Text}
	case token.Integer:
		p.advance()
		e = &ast.IntegerLiteral{Base: ast.Base{SourceSpan: t.Span}, Value: strings.ReplaceAll(t.Text, "_", "")}
	case token.Decimal:
		p.advance()
		e = &ast.DecimalLiteral{Base: ast.Base{SourceSpan: t.Span}, Value: strings.ReplaceAll(t.Text, "_", "")}
	case token.True, token.False:
		p.advance()
		e = &ast.BooleanLiteral{Base: ast.Base{SourceSpan: t.Span}, Value: t.Kind == token.True}
	case token.Null:
		p.advance()
		e = &ast.NullLiteral{Base: ast.Base{SourceSpan: t.Span}}
	case token.Bang, token.Minus:
		p.advance()
		v := p.expression(7)
		if v == nil {
			return nil
		}
		e = &ast.UnaryExpression{Base: base(t.Span, v.Span()), Operator: t.Text, Operand: v}
	case token.LeftParen:
		p.advance()
		v := p.expression(0)
		end := p.expect(token.RightParen, "`)`").Span
		e = &ast.ParenthesisedExpression{Base: base(t.Span, end), Expression: v}
	case token.LeftBrace:
		e = p.object()
	case token.LeftBracket:
		e = p.list()
	default:
		if t.Kind.IsKeyword() {
			p.advance()
			e = &ast.IdentifierExpression{Base: ast.Base{SourceSpan: t.Span}, Name: t.Text}
			break
		}
		p.report("PAR003", "expected expression", t.Span)
		return nil
	}
	for {
		if p.match(token.Dot) {
			m := p.cur()
			if m.Kind != token.Identifier && !m.Kind.IsKeyword() {
				p.report("PAR002", "expected member name", m.Span)
			} else {
				p.advance()
			}
			e = &ast.MemberExpression{Base: base(e.Span(), m.Span), Object: e, Member: m.Text}
			continue
		}
		if p.match(token.LeftBracket) {
			i := p.expression(0)
			end := p.expect(token.RightBracket, "`]`").Span
			e = &ast.IndexExpression{Base: base(e.Span(), end), Object: e, Index: i}
			continue
		}
		if p.match(token.LeftParen) {
			var args []ast.Expression
			for !p.at(token.RightParen) && !p.at(token.EOF) {
				a := p.expression(0)
				if a == nil {
					break
				}
				args = append(args, a)
				if !p.match(token.Comma) && p.at(token.RightParen) {
					break
				}
			}
			end := p.expect(token.RightParen, "`)`").Span
			e = &ast.CallExpression{Base: base(e.Span(), end), Callee: e, Arguments: args}
			continue
		}
		break
	}
	return e
}
func (p *parser) object() ast.Expression {
	open := p.advance()
	var es []ast.ObjectEntry
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		k := p.cur()
		if k.Kind != token.Identifier && k.Kind != token.String {
			p.report("PAR007", "invalid object entry", k.Span)
			p.syncStatement()
			continue
		}
		p.advance()
		p.expect(token.Equal, "`=`")
		v := p.expression(0)
		if v == nil {
			break
		}
		es = append(es, ast.ObjectEntry{Base: base(k.Span, v.Span()), Key: func() string {
			if k.Kind == token.String {
				return k.Value
			}
			return k.Text
		}(), Quoted: k.Kind == token.String, Value: v})
		p.match(token.Comma)
	}
	end := p.expect(token.RightBrace, "`}`").Span
	return &ast.ObjectExpression{Base: base(open.Span, end), Entries: es}
}
func (p *parser) list() ast.Expression {
	open := p.advance()
	var es []ast.Expression
	for !p.at(token.RightBracket) && !p.at(token.EOF) {
		v := p.expression(0)
		if v == nil {
			break
		}
		es = append(es, v)
		p.match(token.Comma)
	}
	end := p.expect(token.RightBracket, "`]`").Span
	return &ast.ListExpression{Base: base(open.Span, end), Elements: es}
}
func (p *parser) syncDeclaration() {
	for !p.at(token.EOF) {
		switch p.cur().Kind {
		case token.KeywordType, token.KeywordEnum, token.KeywordModule, token.KeywordUse, token.KeywordVariant, token.KeywordEnvironment, token.KeywordTransform, token.KeywordPolicy, token.KeywordTest:
			return
		}
		p.advance()
	}
}
func (p *parser) syncStatement() {
	for !p.at(token.EOF) && !p.at(token.RightBrace) {
		if p.cur().Kind.IsKeyword() || p.cur().Kind == token.Identifier {
			return
		}
		p.advance()
	}
}

// Tree returns a stable developer-facing representation.
func Tree(f *ast.File) string {
	var b strings.Builder
	fmt.Fprintf(&b, "File %s\n", f.Name)
	for i, d := range f.Declarations {
		last := i == len(f.Declarations)-1
		prefix := "├── "
		if last {
			prefix = "└── "
		}
		fmt.Fprintf(&b, "%s%T\n", prefix, d)
	}
	return strings.ReplaceAll(b.String(), "*ast.", "")
}
