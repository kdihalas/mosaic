package diagnostics

import "sort"

// List is an ordered collection of diagnostics.
type List []Diagnostic

// HasErrors reports whether the list contains at least one error diagnostic.
func (l List) HasErrors() bool {
	for _, diagnostic := range l {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Sorted returns a stable copy ordered by source position, severity, and code.
func (l List) Sorted() List {
	out := append(List(nil), l...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Span.SourceName != b.Span.SourceName {
			return a.Span.SourceName < b.Span.SourceName
		}
		if a.Span.Start.Offset != b.Span.Start.Offset {
			return a.Span.Start.Offset < b.Span.Start.Offset
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		return a.Code < b.Code
	})
	return out
}

// Append returns l with the supplied lists appended.
func (l List) Append(other ...List) List {
	for _, x := range other {
		l = append(l, x...)
	}
	return l
}
