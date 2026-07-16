// Package module exposes immutable module instance metadata.
package module

import "github.com/kdihalas/mosaic/pkg/graph"

type Instance struct {
	Module    string             `json:"module"`
	Alias     string             `json:"alias"`
	Resources []graph.ResourceID `json:"resources"`
	Exports   []Export           `json:"exports,omitempty"`
}

// Export describes the instantiated visibility of a module resource.
type Export struct {
	ResourceID graph.ResourceID `json:"resourceId"`
	Optional   bool             `json:"optional,omitempty"`
	Present    bool             `json:"present"`
}
