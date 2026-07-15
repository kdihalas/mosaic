// Package transform defines typed graph writes and conflict records.
package transform

import (
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/provenance"
	"github.com/kdihalas/mosaic/pkg/value"
)

type OperationKind string

const (
	Set     OperationKind = "set"
	Replace OperationKind = "replace"
	Delete  OperationKind = "delete"
	Append  OperationKind = "append"
	Merge   OperationKind = "merge"
	Add     OperationKind = "add"
	Enable  OperationKind = "enable"
	Resolve OperationKind = "resolve"
)

type FieldWrite struct {
	ResourceID graph.ResourceID `json:"resourceId"`
	Path       graph.FieldPath  `json:"path"`
	Operation  OperationKind    `json:"operation"`
	Value      value.Value      `json:"value"`
	Owner      provenance.Owner `json:"owner"`
	Source     diagnostics.Span `json:"source"`
	Explicit   bool             `json:"explicit"`
}
type Conflict struct {
	ResourceID graph.ResourceID `json:"resourceId"`
	Path       graph.FieldPath  `json:"path"`
	Writes     []FieldWrite     `json:"writes"`
	Resolved   bool             `json:"resolved"`
}
