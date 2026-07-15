// Package provenance records deterministic resource and field histories.
package provenance

import (
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/value"
	"sort"
	"sync"
)

type Action string

const (
	ResourceCreated   Action = "resource-created"
	DefaultApplied    Action = "default-applied"
	InputSupplied     Action = "input-supplied"
	ModuleAssigned    Action = "module-assigned"
	VariantSet        Action = "variant-set"
	TransformApplied  Action = "transform-applied"
	CapabilityEnabled Action = "capability-enabled"
	ConflictResolved  Action = "conflict-resolved"
	PolicyValidated   Action = "policy-validated"
	RendererDerived   Action = "renderer-derived"
)

type Owner struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}
type Event struct {
	Sequence   int              `json:"sequence"`
	ResourceID graph.ResourceID `json:"resourceId"`
	FieldPath  graph.FieldPath  `json:"fieldPath,omitempty"`
	Action     Action           `json:"action"`
	Previous   value.Value      `json:"previous"`
	Current    value.Value      `json:"current"`
	Owner      Owner            `json:"owner"`
	Source     diagnostics.Span `json:"source"`
}
type Store struct {
	mu     sync.RWMutex
	events []Event
}

func New() *Store { return &Store{} }
func (s *Store) Add(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.Sequence = len(s.events) + 1
	e.FieldPath = e.FieldPath.Clone()
	e.Previous = e.Previous.Clone()
	e.Current = e.Current.Clone()
	s.events = append(s.events, e)
}
func (s *Store) Events() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	x := append([]Event(nil), s.events...)
	return x
}
func (s *Store) ResourceHistory(id graph.ResourceID) []Event {
	var x []Event
	for _, e := range s.Events() {
		if e.ResourceID == id {
			x = append(x, e)
		}
	}
	return x
}
func (s *Store) FieldHistory(id graph.ResourceID, p graph.FieldPath) []Event {
	var x []Event
	for _, e := range s.Events() {
		if e.ResourceID == id && e.FieldPath.String() == p.String() {
			x = append(x, e)
		}
	}
	sort.SliceStable(x, func(i, j int) bool { return x[i].Sequence < x[j].Sequence })
	return x
}
