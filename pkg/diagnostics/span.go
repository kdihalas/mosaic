package diagnostics

// Span identifies an end-exclusive range in a named source.
type Span struct {
	SourceName string   `json:"sourceName"`
	Start      Position `json:"start"`
	End        Position `json:"end"`
}
