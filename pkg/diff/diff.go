// Package diff computes semantic graph differences.
package diff

import (
	"encoding/json"
	"fmt"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/value"
	"sort"
	"strings"
)

type Kind string

const (
	EnvironmentChanged Kind = "environment-changed"
	ResourceAdded      Kind = "resource-added"
	ResourceRemoved    Kind = "resource-removed"
	FieldAdded         Kind = "field-added"
	FieldRemoved       Kind = "field-removed"
	ScalarChanged      Kind = "scalar-changed"
	CapabilityAdded    Kind = "capability-added"
	CapabilityRemoved  Kind = "capability-removed"
)

type Change struct {
	Kind       Kind             `json:"kind"`
	ResourceID graph.ResourceID `json:"resourceId,omitempty"`
	Path       graph.FieldPath  `json:"path,omitempty"`
	Before     value.Value      `json:"before"`
	After      value.Value      `json:"after"`
}
type Result struct {
	OldEnvironment string   `json:"oldEnvironment"`
	NewEnvironment string   `json:"newEnvironment"`
	Changes        []Change `json:"changes"`
}

func Compare(oldEnv, newEnv string, a, b *graph.Graph) Result {
	r := Result{OldEnvironment: oldEnv, NewEnvironment: newEnv}
	am := map[graph.ResourceID]graph.Resource{}
	bm := map[graph.ResourceID]graph.Resource{}
	for _, x := range a.List() {
		am[x.ID] = x
	}
	for _, x := range b.List() {
		bm[x.ID] = x
	}
	ids := map[graph.ResourceID]bool{}
	for id := range am {
		ids[id] = true
	}
	for id := range bm {
		ids[id] = true
	}
	order := make([]string, 0, len(ids))
	for id := range ids {
		order = append(order, string(id))
	}
	sort.Strings(order)
	for _, s := range order {
		id := graph.ResourceID(s)
		x, xok := am[id]
		y, yok := bm[id]
		if !xok {
			k := ResourceAdded
			if strings.Contains(s, ".capability.") {
				k = CapabilityAdded
			}
			r.Changes = append(r.Changes, Change{Kind: k, ResourceID: id, After: y.Fields})
			continue
		}
		if !yok {
			k := ResourceRemoved
			if strings.Contains(s, ".capability.") {
				k = CapabilityRemoved
			}
			r.Changes = append(r.Changes, Change{Kind: k, ResourceID: id, Before: x.Fields})
			continue
		}
		walk(&r, id, nil, x.Fields, y.Fields)
	}
	return r
}
func walk(r *Result, id graph.ResourceID, p graph.FieldPath, a, b value.Value) {
	if a.Equal(b) {
		return
	}
	if a.Kind() == value.KindObject && b.Kind() == value.KindObject {
		keys := map[string]bool{}
		for _, k := range a.Keys() {
			keys[k] = true
		}
		for _, k := range b.Keys() {
			keys[k] = true
		}
		ks := make([]string, 0, len(keys))
		for k := range keys {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			x, xok := a.Get(k)
			y, yok := b.Get(k)
			path := append(p.Clone(), k)
			if !xok {
				r.Changes = append(r.Changes, Change{Kind: FieldAdded, ResourceID: id, Path: path, After: y})
			} else if !yok {
				r.Changes = append(r.Changes, Change{Kind: FieldRemoved, ResourceID: id, Path: path, Before: x})
			} else {
				walk(r, id, path, x, y)
			}
		}
		return
	}
	r.Changes = append(r.Changes, Change{Kind: ScalarChanged, ResourceID: id, Path: p, Before: a, After: b})
}
func (r Result) JSON() []byte { b, _ := json.MarshalIndent(r, "", "  "); return append(b, '\n') }
func (r Result) Text() string {
	var b strings.Builder
	if r.OldEnvironment != r.NewEnvironment {
		fmt.Fprintf(&b, "Environment:\n  %s → %s\n\n", r.OldEnvironment, r.NewEnvironment)
	}
	for _, c := range r.Changes {
		fmt.Fprintf(&b, "%s: %s", c.Kind, c.ResourceID)
		if len(c.Path) > 0 {
			fmt.Fprintf(&b, ".%s", c.Path.String())
		}
		if c.Kind == ScalarChanged {
			x, _ := c.Before.CanonicalJSON()
			y, _ := c.After.CanonicalJSON()
			fmt.Fprintf(&b, "\n  %s → %s", x, y)
		}
		b.WriteString("\n")
	}
	return b.String()
}
