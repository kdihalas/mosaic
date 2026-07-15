// Package diagnostics defines source locations and structured compiler diagnostics.
package diagnostics

// Position identifies a location in source. Offset is a zero-based byte
// offset; Line and Column are one-based.
type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}
