// Package types defines Mosaic static types.
package types

import "strings"

type Kind string

const (
	Null      Kind = "null"
	Bool      Kind = "bool"
	Int       Kind = "int"
	Decimal   Kind = "decimal"
	String    Kind = "string"
	List      Kind = "list"
	Map       Kind = "map"
	Object    Kind = "object"
	Enum      Kind = "enum"
	Optional  Kind = "optional"
	Resource  Kind = "resource"
	Reference Kind = "reference"
)

type Type struct {
	Name      string          `json:"name"`
	Kind      Kind            `json:"kind"`
	Arguments []Type          `json:"arguments,omitempty"`
	Fields    map[string]Type `json:"fields,omitempty"`
	Members   []string        `json:"members,omitempty"`
}

func (t Type) String() string {
	if len(t.Arguments) == 0 {
		return t.Name
	}
	x := make([]string, len(t.Arguments))
	for i := range t.Arguments {
		x[i] = t.Arguments[i].String()
	}
	return t.Name + "<" + strings.Join(x, ", ") + ">"
}

var Builtins = map[string]Type{"null": {Name: "null", Kind: Null}, "bool": {Name: "bool", Kind: Bool}, "int": {Name: "int", Kind: Int}, "decimal": {Name: "decimal", Kind: Decimal}, "string": {Name: "string", Kind: String}, "ImageReference": {Name: "ImageReference", Kind: String}, "CpuQuantity": {Name: "CpuQuantity", Kind: String}, "MemoryQuantity": {Name: "MemoryQuantity", Kind: String}, "Duration": {Name: "Duration", Kind: String}, "PortNumber": {Name: "PortNumber", Kind: Int}, "ResourceName": {Name: "ResourceName", Kind: String}}
