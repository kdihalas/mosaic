package diagnostics

// Severity is the importance of a diagnostic.
type Severity string

const (
	// SeverityError identifies a source error.
	SeverityError Severity = "error"
	// SeverityWarning identifies a non-fatal warning.
	SeverityWarning Severity = "warning"
)

// Diagnostic is a structured problem associated with a source span.
type Diagnostic struct {
	Code       string    `json:"code"`
	Severity   Severity  `json:"severity"`
	Message    string    `json:"message"`
	Span       Span      `json:"span"`
	Notes      []string  `json:"notes,omitempty"`
	Related    []Related `json:"related,omitempty"`
	Suggestion string    `json:"suggestion,omitempty"`
}

// Related identifies a secondary location relevant to a diagnostic.
type Related struct {
	Message string `json:"message"`
	Span    Span   `json:"span"`
}
