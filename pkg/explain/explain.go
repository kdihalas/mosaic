// Package explain queries compiler provenance.
package explain

import (
	"fmt"
	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/provenance"
	"github.com/kdihalas/mosaic/pkg/value"
)

type Result struct {
	Resource graph.Resource     `json:"resource"`
	Path     graph.FieldPath    `json:"path,omitempty"`
	Value    value.Value        `json:"value"`
	History  []provenance.Event `json:"history"`
}

func Query(r *compiler.Result, id graph.ResourceID, path graph.FieldPath) (Result, error) {
	res, ok := r.Graph.Get(id)
	if !ok {
		return Result{}, fmt.Errorf("resource %s does not exist", id)
	}
	out := Result{Resource: res, Path: path, History: r.Provenance.ResourceHistory(id)}
	if len(path) > 0 {
		v, ok := r.Graph.ReadField(id, path)
		if !ok {
			return Result{}, fmt.Errorf("field %s does not exist", path.String())
		}
		out.Value = v
		out.History = r.Provenance.FieldHistory(id, path)
	} else {
		out.Value = res.Fields
	}
	return out, nil
}
