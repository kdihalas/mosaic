// Package value implements Mosaic's precise runtime value model.
package value

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
)

type Kind uint8

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindDecimal
	KindString
	KindList
	KindObject
	KindReference
)

type ReferenceValue struct {
	Target string           `json:"target"`
	Field  []string         `json:"field,omitempty"`
	Type   string           `json:"type,omitempty"`
	Source diagnostics.Span `json:"source"`
}
type Value struct {
	kind   Kind
	b      bool
	i      *big.Int
	d      *big.Rat
	s      string
	list   []Value
	object map[string]Value
	ref    *ReferenceValue
}

func Null() Value       { return Value{kind: KindNull} }
func Bool(v bool) Value { return Value{kind: KindBool, b: v} }
func Int(v *big.Int) Value {
	if v == nil {
		return Null()
	}
	return Value{kind: KindInt, i: new(big.Int).Set(v)}
}
func Decimal(v *big.Rat) Value {
	if v == nil {
		return Null()
	}
	return Value{kind: KindDecimal, d: new(big.Rat).Set(v)}
}
func String(v string) Value { return Value{kind: KindString, s: v} }
func List(v []Value) Value {
	r := make([]Value, len(v))
	for i := range v {
		r[i] = v[i].Clone()
	}
	return Value{kind: KindList, list: r}
}
func Object(v map[string]Value) Value {
	r := make(map[string]Value, len(v))
	for k, x := range v {
		r[k] = x.Clone()
	}
	return Value{kind: KindObject, object: r}
}
func Reference(v ReferenceValue) Value {
	x := v
	x.Field = append([]string(nil), v.Field...)
	return Value{kind: KindReference, ref: &x}
}
func (v Value) Kind() Kind                  { return v.kind }
func (v Value) BoolValue() (bool, bool)     { return v.b, v.kind == KindBool }
func (v Value) StringValue() (string, bool) { return v.s, v.kind == KindString }
func (v Value) IntValue() (*big.Int, bool) {
	if v.kind != KindInt {
		return nil, false
	}
	return new(big.Int).Set(v.i), true
}
func (v Value) DecimalValue() (*big.Rat, bool) {
	if v.kind != KindDecimal {
		return nil, false
	}
	return new(big.Rat).Set(v.d), true
}
func (v Value) ListValue() ([]Value, bool) {
	if v.kind != KindList {
		return nil, false
	}
	return List(v.list).list, true
}
func (v Value) ObjectValue() (map[string]Value, bool) {
	if v.kind != KindObject {
		return nil, false
	}
	return Object(v.object).object, true
}
func (v Value) ReferenceValue() (ReferenceValue, bool) {
	if v.kind != KindReference || v.ref == nil {
		return ReferenceValue{}, false
	}
	return *Reference(*v.ref).ref, true
}
func (v Value) Get(key string) (Value, bool) {
	if v.kind != KindObject {
		return Value{}, false
	}
	x, ok := v.object[key]
	return x.Clone(), ok
}
func (v Value) Keys() []string {
	if v.kind != KindObject {
		return nil
	}
	ks := make([]string, 0, len(v.object))
	for k := range v.object {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
func (v Value) Equal(o Value) bool {
	a, _ := v.CanonicalJSON()
	b, _ := o.CanonicalJSON()
	return bytes.Equal(a, b)
}
func (v Value) Clone() Value {
	switch v.kind {
	case KindInt:
		return Int(v.i)
	case KindDecimal:
		return Decimal(v.d)
	case KindList:
		return List(v.list)
	case KindObject:
		return Object(v.object)
	case KindReference:
		return Reference(*v.ref)
	default:
		return v
	}
}
func (v Value) With(key string, x Value) (Value, error) {
	if v.kind != KindObject {
		return Value{}, errors.New("value is not an object")
	}
	m, _ := v.ObjectValue()
	m[key] = x.Clone()
	return Object(m), nil
}
func (v Value) Without(key string) (Value, error) {
	if v.kind != KindObject {
		return Value{}, errors.New("value is not an object")
	}
	m, _ := v.ObjectValue()
	delete(m, key)
	return Object(m), nil
}
func (v Value) CanonicalJSON() ([]byte, error) {
	var b bytes.Buffer
	err := v.write(&b)
	return b.Bytes(), err
}
func (v Value) MarshalJSON() ([]byte, error) { return v.CanonicalJSON() }
func (v *Value) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var x any
	if err := dec.Decode(&x); err != nil {
		return err
	}
	y, err := fromNative(x)
	if err != nil {
		return err
	}
	*v = y
	return nil
}
func fromNative(x any) (Value, error) {
	switch z := x.(type) {
	case nil:
		return Null(), nil
	case bool:
		return Bool(z), nil
	case string:
		return String(z), nil
	case json.Number:
		if strings.ContainsAny(string(z), ".eE") {
			r, ok := new(big.Rat).SetString(string(z))
			if !ok {
				return Value{}, errors.New("invalid decimal")
			}
			return Decimal(r), nil
		}
		i, ok := new(big.Int).SetString(string(z), 10)
		if !ok {
			return Value{}, errors.New("invalid integer")
		}
		return Int(i), nil
	case []any:
		a := make([]Value, len(z))
		for i := range z {
			v, e := fromNative(z[i])
			if e != nil {
				return Value{}, e
			}
			a[i] = v
		}
		return List(a), nil
	case map[string]any:
		if raw, ok := z["$reference"].(map[string]any); ok {
			r := ReferenceValue{}
			if s, yes := raw["target"].(string); yes {
				r.Target = s
			}
			if s, yes := raw["type"].(string); yes {
				r.Type = s
			}
			if a, yes := raw["field"].([]any); yes {
				for _, q := range a {
					if s, ok := q.(string); ok {
						r.Field = append(r.Field, s)
					}
				}
			}
			return Reference(r), nil
		}
		m := map[string]Value{}
		for k, q := range z {
			v, e := fromNative(q)
			if e != nil {
				return Value{}, e
			}
			m[k] = v
		}
		return Object(m), nil
	}
	return Value{}, errors.New("unsupported JSON value")
}
func (v Value) write(b *bytes.Buffer) error {
	switch v.kind {
	case KindNull:
		b.WriteString("null")
	case KindBool:
		if v.b {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case KindInt:
		b.WriteString(v.i.String())
	case KindDecimal:
		b.WriteString(decimal(v.d))
	case KindString:
		x, _ := json.Marshal(v.s)
		b.Write(x)
	case KindList:
		b.WriteByte('[')
		for i, x := range v.list {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := x.write(b); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case KindObject:
		b.WriteByte('{')
		ks := v.Keys()
		for i, k := range ks {
			if i > 0 {
				b.WriteByte(',')
			}
			x, _ := json.Marshal(k)
			b.Write(x)
			b.WriteByte(':')
			if err := v.object[k].write(b); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case KindReference:
		b.WriteString(`{"$reference":`)
		x, _ := json.Marshal(v.ref)
		b.Write(x)
		b.WriteByte('}')
	default:
		return errors.New("invalid value kind")
	}
	return nil
}
func decimal(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	den := new(big.Int).Set(r.Denom())
	two := 0
	five := 0
	z := big.NewInt(0)
	for new(big.Int).Mod(den, big.NewInt(2)).Cmp(z) == 0 {
		den.Div(den, big.NewInt(2))
		two++
	}
	for new(big.Int).Mod(den, big.NewInt(5)).Cmp(z) == 0 {
		den.Div(den, big.NewInt(5))
		five++
	}
	if den.Cmp(big.NewInt(1)) != 0 {
		return strconv.Quote(r.RatString())
	}
	n := two
	if five > n {
		n = five
	}
	s := r.FloatString(n)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
