// Package ast defines Mosaic's lossless-span syntax tree.
package ast

import (
	"encoding/json"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/syntax/token"
)

type Node interface{ Span() diagnostics.Span }
type Declaration interface {
	Node
	declarationNode()
}
type Statement interface {
	Node
	statementNode()
}
type Expression interface {
	Node
	expressionNode()
}

type Base struct {
	SourceSpan diagnostics.Span `json:"span"`
}

func (b Base) Span() diagnostics.Span { return b.SourceSpan }

type File struct {
	Name         string        `json:"name"`
	Declarations []Declaration `json:"declarations"`
	Tokens       []token.Token `json:"-"`
}

type TypeField struct {
	Base
	Name    string     `json:"name"`
	Type    Expression `json:"type"`
	Default Expression `json:"default,omitempty"`
}
type TypeDeclaration struct {
	Base
	Name   string      `json:"name"`
	Fields []TypeField `json:"fields"`
	Body   []Statement `json:"body"`
}
type EnumDeclaration struct {
	Base
	Name    string                 `json:"name"`
	Members []IdentifierExpression `json:"members"`
}
type Parameter struct {
	Base
	Name string     `json:"name"`
	Type Expression `json:"type"`
}
type ModuleDeclaration struct {
	Base
	Name      string      `json:"name"`
	Parameter *Parameter  `json:"parameter,omitempty"`
	Body      []Statement `json:"body"`
}
type ModuleUseDeclaration struct {
	Base
	Module string      `json:"module"`
	Alias  string      `json:"alias"`
	Body   []Statement `json:"body"`
}
type VariantDeclaration struct {
	Base
	Name string      `json:"name"`
	Body []Statement `json:"body"`
}
type EnvironmentDeclaration struct {
	Base
	Name string      `json:"name"`
	Body []Statement `json:"body"`
}
type TransformDeclaration struct {
	Base
	Name string      `json:"name"`
	Body []Statement `json:"body"`
}
type PolicyDeclaration struct {
	Base
	Name string      `json:"name"`
	Body []Statement `json:"body"`
}
type TestDeclaration struct {
	Base
	Name string      `json:"name"`
	Body []Statement `json:"body"`
}

func (*TypeDeclaration) declarationNode()        {}
func (*EnumDeclaration) declarationNode()        {}
func (*ModuleDeclaration) declarationNode()      {}
func (*ModuleUseDeclaration) declarationNode()   {}
func (*VariantDeclaration) declarationNode()     {}
func (*EnvironmentDeclaration) declarationNode() {}
func (*TransformDeclaration) declarationNode()   {}
func (*PolicyDeclaration) declarationNode()      {}
func (*TestDeclaration) declarationNode()        {}

// Statements intentionally retain contextual operation names. Semantic phases
// turn these syntax forms into typed operations.
type AssignmentStatement struct {
	Base
	Target Expression `json:"target"`
	Value  Expression `json:"value"`
}
type ResourceDeclaration struct {
	Base
	Kind string      `json:"kind"`
	Name string      `json:"name"`
	Body []Statement `json:"body"`
}
type BlockDeclaration struct {
	Base
	Name  string      `json:"name"`
	Label string      `json:"label,omitempty"`
	Body  []Statement `json:"body"`
}
type UseStatement struct {
	Base
	Name string `json:"name"`
}
type ApplyStatement struct {
	Base
	Name string `json:"name"`
}
type OperationStatement struct {
	Base
	Operation string      `json:"operation"`
	Target    Expression  `json:"target,omitempty"`
	Value     Expression  `json:"value,omitempty"`
	Identity  string      `json:"identity,omitempty"`
	Name      string      `json:"name,omitempty"`
	Body      []Statement `json:"body,omitempty"`
}
type EnableStatement OperationStatement
type SetStatement OperationStatement
type ReplaceStatement OperationStatement
type DeleteStatement OperationStatement
type AppendStatement OperationStatement
type MergeStatement OperationStatement
type AddStatement OperationStatement
type ResolveStatement OperationStatement
type ExportStatement struct {
	Base
	Target Expression `json:"target"`
}
type ExtensionStatement struct {
	Base
	Target Expression `json:"target"`
}
type ProtectedStatement struct {
	Base
	Target Expression `json:"target"`
}
type RequireStatement struct {
	Base
	Condition Expression  `json:"condition"`
	Body      []Statement `json:"body,omitempty"`
}
type DenyStatement RequireStatement
type WarnStatement RequireStatement
type ExpressionStatement struct {
	Base
	Expression Expression `json:"expression"`
}
type SelectStatement struct {
	Base
	Type  string      `json:"type"`
	Where Expression  `json:"where,omitempty"`
	Body  []Statement `json:"body"`
}

func (*AssignmentStatement) statementNode() {}
func (*ResourceDeclaration) statementNode() {}
func (*BlockDeclaration) statementNode()    {}
func (*UseStatement) statementNode()        {}
func (*ApplyStatement) statementNode()      {}
func (*OperationStatement) statementNode()  {}
func (*EnableStatement) statementNode()     {}
func (*SetStatement) statementNode()        {}
func (*ReplaceStatement) statementNode()    {}
func (*DeleteStatement) statementNode()     {}
func (*AppendStatement) statementNode()     {}
func (*MergeStatement) statementNode()      {}
func (*AddStatement) statementNode()        {}
func (*ResolveStatement) statementNode()    {}
func (*ExportStatement) statementNode()     {}
func (*ExtensionStatement) statementNode()  {}
func (*ProtectedStatement) statementNode()  {}
func (*RequireStatement) statementNode()    {}
func (*DenyStatement) statementNode()       {}
func (*WarnStatement) statementNode()       {}
func (*ExpressionStatement) statementNode() {}
func (*SelectStatement) statementNode()     {}

type IdentifierExpression struct {
	Base
	Name string `json:"name"`
}
type StringLiteral struct {
	Base
	Value string `json:"value"`
	Raw   string `json:"raw"`
}
type IntegerLiteral struct {
	Base
	Value string `json:"value"`
}
type DecimalLiteral struct {
	Base
	Value string `json:"value"`
}
type BooleanLiteral struct {
	Base
	Value bool `json:"value"`
}
type NullLiteral struct{ Base }
type MemberExpression struct {
	Base
	Object Expression `json:"object"`
	Member string     `json:"member"`
}
type IndexExpression struct {
	Base
	Object Expression `json:"object"`
	Index  Expression `json:"index"`
}
type CallExpression struct {
	Base
	Callee    Expression   `json:"callee"`
	Arguments []Expression `json:"arguments"`
}
type ObjectEntry struct {
	Base
	Key    string     `json:"key"`
	Quoted bool       `json:"quoted"`
	Value  Expression `json:"value"`
}
type ObjectExpression struct {
	Base
	Entries []ObjectEntry `json:"entries"`
}
type ListExpression struct {
	Base
	Elements []Expression `json:"elements"`
}
type UnaryExpression struct {
	Base
	Operator string     `json:"operator"`
	Operand  Expression `json:"operand"`
}
type BinaryExpression struct {
	Base
	Left     Expression `json:"left"`
	Operator string     `json:"operator"`
	Right    Expression `json:"right"`
}
type ParenthesisedExpression struct {
	Base
	Expression Expression `json:"expression"`
}
type ConstructExpression struct {
	Base
	Type Expression    `json:"type"`
	Body []ObjectEntry `json:"body"`
}

func (*IdentifierExpression) expressionNode()    {}
func (*StringLiteral) expressionNode()           {}
func (*IntegerLiteral) expressionNode()          {}
func (*DecimalLiteral) expressionNode()          {}
func (*BooleanLiteral) expressionNode()          {}
func (*NullLiteral) expressionNode()             {}
func (*MemberExpression) expressionNode()        {}
func (*IndexExpression) expressionNode()         {}
func (*CallExpression) expressionNode()          {}
func (*ObjectExpression) expressionNode()        {}
func (*ListExpression) expressionNode()          {}
func (*UnaryExpression) expressionNode()         {}
func (*BinaryExpression) expressionNode()        {}
func (*ParenthesisedExpression) expressionNode() {}
func (*ConstructExpression) expressionNode()     {}

// MarshalJSON emits declarations through their concrete representations.
func (f File) MarshalJSON() ([]byte, error) {
	type wire struct {
		Name         string `json:"name"`
		Declarations []any  `json:"declarations"`
	}
	d := make([]any, len(f.Declarations))
	for i := range f.Declarations {
		d[i] = f.Declarations[i]
	}
	return json.Marshal(wire{f.Name, d})
}
