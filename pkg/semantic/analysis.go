// Package semantic implements deterministic expression evaluation and analysis results.
package semantic

import (
	"fmt"
	"github.com/kdihalas/mosaic/pkg/semantic/symbols"
	"github.com/kdihalas/mosaic/pkg/syntax/ast"
	"github.com/kdihalas/mosaic/pkg/value"
	"math/big"
	"regexp"
	"strings"
)

type Analysis struct {
	Files   []*ast.File      `json:"files"`
	Symbols []symbols.Symbol `json:"symbols"`
}
type Context struct {
	Values      map[string]value.Value
	ResolvePath func([]string) (value.Value, bool)
}

func Path(e ast.Expression) ([]string, bool) {
	switch x := e.(type) {
	case *ast.IdentifierExpression:
		return []string{x.Name}, true
	case *ast.MemberExpression:
		p, ok := Path(x.Object)
		return append(p, x.Member), ok
	case *ast.IndexExpression:
		p, ok := Path(x.Object)
		if !ok {
			return nil, false
		}
		key, ok := x.Index.(*ast.StringLiteral)
		if !ok {
			return nil, false
		}
		return append(p, key.Value), true
	default:
		return nil, false
	}
}
func Evaluate(e ast.Expression, c Context) (value.Value, error) {
	switch x := e.(type) {
	case *ast.NullLiteral:
		return value.Null(), nil
	case *ast.BooleanLiteral:
		return value.Bool(x.Value), nil
	case *ast.StringLiteral:
		return value.String(x.Value), nil
	case *ast.IntegerLiteral:
		n, ok := new(big.Int).SetString(x.Value, 10)
		if !ok {
			return value.Value{}, fmt.Errorf("invalid integer")
		}
		return value.Int(n), nil
	case *ast.DecimalLiteral:
		r, ok := new(big.Rat).SetString(x.Value)
		if !ok {
			return value.Value{}, fmt.Errorf("invalid decimal")
		}
		return value.Decimal(r), nil
	case *ast.IdentifierExpression:
		if v, ok := c.Values[x.Name]; ok {
			return v.Clone(), nil
		}
		if c.ResolvePath != nil {
			if v, ok := c.ResolvePath([]string{x.Name}); ok {
				return v, nil
			}
		}
		return value.String(x.Name), nil
	case *ast.MemberExpression:
		if p, ok := Path(x); ok && c.ResolvePath != nil {
			if v, yes := c.ResolvePath(p); yes {
				return v, nil
			}
		}
		o, err := Evaluate(x.Object, c)
		if err != nil {
			return value.Value{}, err
		}
		if v, ok := o.Get(x.Member); ok {
			return v, nil
		}
		return value.Null(), nil
	case *ast.IndexExpression:
		o, err := Evaluate(x.Object, c)
		if err != nil {
			return value.Value{}, err
		}
		k, err := Evaluate(x.Index, c)
		if err != nil {
			return value.Value{}, err
		}
		if list, ok := o.ListValue(); ok {
			index, isInt := k.IntValue()
			if !isInt || !index.IsInt64() {
				return value.Null(), nil
			}
			i := index.Int64()
			if i >= 0 && i < int64(len(list)) {
				return list[i], nil
			}
			return value.Null(), nil
		}
		s, _ := k.StringValue()
		if v, ok := o.Get(s); ok {
			return v, nil
		}
		return value.Null(), nil
	case *ast.ListExpression:
		a := make([]value.Value, len(x.Elements))
		for i, e := range x.Elements {
			v, err := Evaluate(e, c)
			if err != nil {
				return value.Value{}, err
			}
			a[i] = v
		}
		return value.List(a), nil
	case *ast.ObjectExpression:
		m := map[string]value.Value{}
		for _, e := range x.Entries {
			v, err := Evaluate(e.Value, c)
			if err != nil {
				return value.Value{}, err
			}
			m[e.Key] = v
		}
		return value.Object(m), nil
	case *ast.ParenthesisedExpression:
		return Evaluate(x.Expression, c)
	case *ast.UnaryExpression:
		v, err := Evaluate(x.Operand, c)
		if err != nil {
			return value.Value{}, err
		}
		if x.Operator == "!" {
			b, _ := v.BoolValue()
			return value.Bool(!b), nil
		}
		if n, ok := v.IntValue(); ok {
			return value.Int(n.Neg(n)), nil
		}
		if n, ok := v.DecimalValue(); ok {
			return value.Decimal(n.Neg(n)), nil
		}
		return value.Value{}, fmt.Errorf("invalid unary operand")
	case *ast.BinaryExpression:
		return binary(x, c)
	case *ast.CallExpression:
		return call(x, c)
	default:
		return value.Value{}, fmt.Errorf("unsupported expression %T", e)
	}
}
func binary(x *ast.BinaryExpression, c Context) (value.Value, error) {
	a, e := Evaluate(x.Left, c)
	if e != nil {
		return value.Value{}, e
	}
	b, e := Evaluate(x.Right, c)
	if e != nil {
		return value.Value{}, e
	}
	switch x.Operator {
	case "==":
		return value.Bool(a.Equal(b)), nil
	case "!=":
		return value.Bool(!a.Equal(b)), nil
	case "&&", "||":
		av, _ := a.BoolValue()
		bv, _ := b.BoolValue()
		if x.Operator == "&&" {
			return value.Bool(av && bv), nil
		}
		return value.Bool(av || bv), nil
	}
	if as, ok := a.StringValue(); ok {
		bs, _ := b.StringValue()
		switch x.Operator {
		case "+":
			return value.String(as + bs), nil
		case "<":
			return value.Bool(as < bs), nil
		case "<=":
			return value.Bool(as <= bs), nil
		case ">":
			return value.Bool(as > bs), nil
		case ">=":
			return value.Bool(as >= bs), nil
		}
	}
	ar, aok := number(a)
	br, bok := number(b)
	if aok && bok {
		switch x.Operator {
		case "+":
			return value.Decimal(new(big.Rat).Add(ar, br)), nil
		case "-":
			return value.Decimal(new(big.Rat).Sub(ar, br)), nil
		case "*":
			return value.Decimal(new(big.Rat).Mul(ar, br)), nil
		case "/":
			if br.Sign() == 0 {
				return value.Value{}, fmt.Errorf("division by zero")
			}
			return value.Decimal(new(big.Rat).Quo(ar, br)), nil
		case "<":
			return value.Bool(ar.Cmp(br) < 0), nil
		case "<=":
			return value.Bool(ar.Cmp(br) <= 0), nil
		case ">":
			return value.Bool(ar.Cmp(br) > 0), nil
		case ">=":
			return value.Bool(ar.Cmp(br) >= 0), nil
		}
	}
	return value.Value{}, fmt.Errorf("invalid binary operation %s", x.Operator)
}
func number(v value.Value) (*big.Rat, bool) {
	if i, ok := v.IntValue(); ok {
		return new(big.Rat).SetInt(i), true
	}
	return v.DecimalValue()
}
func call(x *ast.CallExpression, c Context) (value.Value, error) {
	m, ok := x.Callee.(*ast.MemberExpression)
	if !ok {
		return value.Value{}, fmt.Errorf("unknown function")
	}
	recv, err := Evaluate(m.Object, c)
	if err != nil {
		return value.Value{}, err
	}
	var arg value.Value
	if len(x.Arguments) > 0 {
		arg, err = Evaluate(x.Arguments[0], c)
		if err != nil {
			return value.Value{}, err
		}
	}
	s, sok := recv.StringValue()
	a, _ := arg.StringValue()
	switch m.Member {
	case "contains":
		return value.Bool(sok && strings.Contains(s, a)), nil
	case "startsWith":
		return value.Bool(sok && strings.HasPrefix(s, a)), nil
	case "endsWith":
		return value.Bool(sok && strings.HasSuffix(s, a)), nil
	case "matches":
		r, e := regexp.Compile(a)
		if e != nil {
			return value.Value{}, e
		}
		return value.Bool(sok && r.MatchString(s)), nil
	case "size":
		if l, ok := recv.ListValue(); ok {
			return value.Int(big.NewInt(int64(len(l)))), nil
		}
		if o, ok := recv.ObjectValue(); ok {
			return value.Int(big.NewInt(int64(len(o)))), nil
		}
	}
	return value.Value{}, fmt.Errorf("unknown function %s", m.Member)
}
