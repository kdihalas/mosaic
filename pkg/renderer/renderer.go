// Package renderer defines backend rendering contracts.
package renderer

import (
	"context"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/provenance"
	"github.com/kdihalas/mosaic/pkg/value"
)

type Renderer interface {
	Name() string
	Render(context.Context, RenderInput) (*ArtifactSet, diagnostics.List)
}
type RenderInput struct {
	Environment string
	Graph       *graph.Graph
	Provenance  *provenance.Store
	Options     value.Value
}
type ArtifactSet struct {
	Files    map[string][]byte `json:"files"`
	Metadata map[string]string `json:"metadata"`
}
