// Package capability names built-in graph capabilities.
package capability

import (
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/module"
)

const (
	Autoscaling          = "autoscaling"
	DisruptionProtection = "disruptionProtection"
)

// Instance records one named package-defined capability expansion.
type Instance struct {
	Capability  string             `json:"capability"`
	Application string             `json:"application"`
	Name        string             `json:"name"`
	Resources   []graph.ResourceID `json:"resources"`
	Exports     []module.Export    `json:"exports,omitempty"`
}
