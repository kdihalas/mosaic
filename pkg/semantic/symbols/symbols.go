// Package symbols defines stable semantic identities.
package symbols

import (
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"sort"
)

type Kind string

const (
	Type        Kind = "type"
	Enum        Kind = "enum"
	Module      Kind = "module"
	Application Kind = "application"
	Variant     Kind = "variant"
	Environment Kind = "environment"
	Transform   Kind = "transform"
	Policy      Kind = "policy"
	Test        Kind = "test"
)

type Symbol struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Kind   Kind             `json:"kind"`
	Source diagnostics.Span `json:"source"`
}
type Table struct{ items map[string]Symbol }

func New() Table { return Table{items: map[string]Symbol{}} }
func (t *Table) Add(s Symbol) bool {
	if _, ok := t.items[s.ID]; ok {
		return false
	}
	t.items[s.ID] = s
	return true
}
func (t Table) Get(id string) (Symbol, bool) { s, ok := t.items[id]; return s, ok }
func (t Table) List() []Symbol {
	x := make([]Symbol, 0, len(t.items))
	for _, s := range t.items {
		x = append(x, s)
	}
	sort.Slice(x, func(i, j int) bool { return x[i].ID < x[j].ID })
	return x
}
