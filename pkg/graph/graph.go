// Package graph defines Mosaic's backend-neutral typed resource graph.
package graph

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/value"
)

type ResourceID string
type TypeName string
type FieldPath []string
type EdgeKind string

const EdgeReference EdgeKind = "reference"

type Metadata struct {
	Module     string           `json:"module,omitempty"`
	Source     diagnostics.Span `json:"source"`
	Exported   bool             `json:"exported,omitempty"`
	Extensions []FieldPath      `json:"extensions,omitempty"`
	Protected  []FieldPath      `json:"protected,omitempty"`
}
type Resource struct {
	ID          ResourceID        `json:"id"`
	Type        TypeName          `json:"type"`
	Name        string            `json:"name"`
	Fields      value.Value       `json:"fields"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Metadata    Metadata          `json:"metadata"`
}
type Edge struct {
	From ResourceID `json:"from"`
	To   ResourceID `json:"to"`
	Kind EdgeKind   `json:"kind"`
}
type Graph struct {
	mu        sync.RWMutex
	resources map[ResourceID]Resource
	edges     []Edge
	frozen    bool
}

func New() *Graph { return &Graph{resources: map[ResourceID]Resource{}} }
func ParseFieldPath(s string) (FieldPath, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ".")
	for _, p := range parts {
		if p == "" {
			return nil, errors.New("empty field path segment")
		}
	}
	return FieldPath(parts), nil
}
func (p FieldPath) String() string   { return strings.Join(p, ".") }
func (p FieldPath) Clone() FieldPath { return append(FieldPath(nil), p...) }
func cloneResource(r Resource) Resource {
	r.Fields = r.Fields.Clone()
	r.Labels = cloneStrings(r.Labels)
	r.Annotations = cloneStrings(r.Annotations)
	r.Metadata.Extensions = clonePaths(r.Metadata.Extensions)
	r.Metadata.Protected = clonePaths(r.Metadata.Protected)
	return r
}
func cloneStrings(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	x := make(map[string]string, len(m))
	for k, v := range m {
		x[k] = v
	}
	return x
}
func clonePaths(p []FieldPath) []FieldPath {
	x := make([]FieldPath, len(p))
	for i := range p {
		x[i] = p[i].Clone()
	}
	return x
}
func (g *Graph) Add(r Resource) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.frozen {
		return errors.New("graph is immutable")
	}
	if r.ID == "" {
		return errors.New("resource ID is required")
	}
	if _, ok := g.resources[r.ID]; ok {
		return errors.New("duplicate resource " + string(r.ID))
	}
	if r.Fields.Kind() != value.KindObject {
		return errors.New("resource fields must be an object")
	}
	g.resources[r.ID] = cloneResource(r)
	return nil
}
func (g *Graph) Remove(id ResourceID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.frozen {
		return errors.New("graph is immutable")
	}
	if _, ok := g.resources[id]; !ok {
		return errors.New("resource not found")
	}
	delete(g.resources, id)
	out := g.edges[:0]
	for _, e := range g.edges {
		if e.From != id && e.To != id {
			out = append(out, e)
		}
	}
	g.edges = out
	return nil
}
func (g *Graph) Get(id ResourceID) (Resource, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	r, ok := g.resources[id]
	return cloneResource(r), ok
}
func (g *Graph) List() []Resource {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := make([]string, 0, len(g.resources))
	for id := range g.resources {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]Resource, len(ids))
	for i, id := range ids {
		out[i] = cloneResource(g.resources[ResourceID(id)])
	}
	return out
}
func (g *Graph) Edges() []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	x := append([]Edge(nil), g.edges...)
	sort.Slice(x, func(i, j int) bool {
		if x[i].From != x[j].From {
			return x[i].From < x[j].From
		}
		if x[i].To != x[j].To {
			return x[i].To < x[j].To
		}
		return x[i].Kind < x[j].Kind
	})
	return x
}
func (g *Graph) AddReference(from, to ResourceID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.frozen {
		return errors.New("graph is immutable")
	}
	if _, ok := g.resources[from]; !ok {
		return errors.New("source resource not found")
	}
	if _, ok := g.resources[to]; !ok {
		return errors.New("target resource not found")
	}
	g.edges = append(g.edges, Edge{from, to, EdgeReference})
	return nil
}
func read(v value.Value, p FieldPath) (value.Value, bool) {
	x := v
	for _, k := range p {
		var ok bool
		x, ok = x.Get(k)
		if !ok {
			return value.Value{}, false
		}
	}
	return x, true
}
func (g *Graph) ReadField(id ResourceID, p FieldPath) (value.Value, bool) {
	r, ok := g.Get(id)
	if !ok {
		return value.Value{}, false
	}
	return read(r.Fields, p)
}

func stringMap(v value.Value) map[string]string {
	m, ok := v.ObjectValue()
	if !ok {
		return nil
	}
	out := make(map[string]string, len(m))
	for key, item := range m {
		if text, ok := item.StringValue(); ok {
			out[key] = text
		}
	}
	return out
}

func syncStringMapFields(r *Resource, p FieldPath) {
	if len(p) == 0 || p[0] == "labels" {
		v, ok := r.Fields.Get("labels")
		if !ok {
			r.Labels = nil
		} else {
			r.Labels = stringMap(v)
		}
	}
	if len(p) == 0 || p[0] == "annotations" {
		v, ok := r.Fields.Get("annotations")
		if !ok {
			r.Annotations = nil
		} else {
			r.Annotations = stringMap(v)
		}
	}
}

func set(v value.Value, p FieldPath, x value.Value) (value.Value, error) {
	if len(p) == 0 {
		return x.Clone(), nil
	}
	child, ok := v.Get(p[0])
	if !ok {
		child = value.Object(map[string]value.Value{})
	}
	next, err := set(child, p[1:], x)
	if err != nil {
		return value.Value{}, err
	}
	return v.With(p[0], next)
}
func del(v value.Value, p FieldPath) (value.Value, error) {
	if len(p) == 0 {
		return value.Value{}, errors.New("cannot delete root")
	}
	if len(p) == 1 {
		return v.Without(p[0])
	}
	child, ok := v.Get(p[0])
	if !ok {
		return v, nil
	}
	next, err := del(child, p[1:])
	if err != nil {
		return value.Value{}, err
	}
	return v.With(p[0], next)
}
func (g *Graph) SetField(id ResourceID, p FieldPath, x value.Value) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.frozen {
		return errors.New("graph is immutable")
	}
	r, ok := g.resources[id]
	if !ok {
		return errors.New("resource not found")
	}
	next, err := set(r.Fields, p, x)
	if err != nil {
		return err
	}
	r.Fields = next
	syncStringMapFields(&r, p)
	g.resources[id] = r
	return nil
}
func (g *Graph) DeleteField(id ResourceID, p FieldPath) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.frozen {
		return errors.New("graph is immutable")
	}
	r, ok := g.resources[id]
	if !ok {
		return errors.New("resource not found")
	}
	next, err := del(r.Fields, p)
	if err != nil {
		return err
	}
	r.Fields = next
	syncStringMapFields(&r, p)
	g.resources[id] = r
	return nil
}
func (g *Graph) Clone() *Graph {
	x := New()
	for _, r := range g.List() {
		x.resources[r.ID] = r
	}
	x.edges = g.Edges()
	return x
}
func (g *Graph) Snapshot() *Graph { x := g.Clone(); x.frozen = true; return x }
func (g *Graph) Validate() error {
	for _, e := range g.Edges() {
		if _, ok := g.Get(e.From); !ok {
			return errors.New("dangling edge source")
		}
		if _, ok := g.Get(e.To); !ok {
			return errors.New("dangling edge target")
		}
	}
	return nil
}
func (g *Graph) CanonicalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`{"resources":[`)
	rs := g.List()
	for i, r := range rs {
		if i > 0 {
			b.WriteByte(',')
		}
		fields, err := r.Fields.CanonicalJSON()
		if err != nil {
			return nil, err
		}
		w := struct {
			ID          ResourceID        `json:"id"`
			Type        TypeName          `json:"type"`
			Name        string            `json:"name"`
			Fields      json.RawMessage   `json:"fields"`
			Labels      map[string]string `json:"labels,omitempty"`
			Annotations map[string]string `json:"annotations,omitempty"`
			Metadata    Metadata          `json:"metadata"`
		}{r.ID, r.Type, r.Name, fields, r.Labels, r.Annotations, r.Metadata}
		x, _ := json.Marshal(w)
		b.Write(x)
	}
	b.WriteString(`],"edges":`)
	x, _ := json.Marshal(g.Edges())
	b.Write(x)
	b.WriteByte('}')
	return b.Bytes(), nil
}

// DecodeCanonical reconstructs a graph from graph.json.
func DecodeCanonical(data []byte) (*Graph, error) {
	var w struct {
		Resources []Resource `json:"resources"`
		Edges     []Edge     `json:"edges"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	g := New()
	for _, r := range w.Resources {
		if err := g.Add(r); err != nil {
			return nil, err
		}
	}
	for _, e := range w.Edges {
		if e.Kind == EdgeReference {
			if err := g.AddReference(e.From, e.To); err != nil {
				return nil, err
			}
		}
	}
	return g, nil
}
