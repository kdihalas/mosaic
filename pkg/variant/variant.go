// Package variant exposes compiled variant metadata.
package variant

type Variant struct {
	Name       string `json:"name"`
	Operations int    `json:"operations"`
}
