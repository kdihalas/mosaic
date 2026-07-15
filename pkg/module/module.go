// Package module exposes immutable module instance metadata.
package module

import "github.com/kdihalas/mosaic/pkg/graph"

type Instance struct {
	Module    string             `json:"module"`
	Alias     string             `json:"alias"`
	Resources []graph.ResourceID `json:"resources"`
}
