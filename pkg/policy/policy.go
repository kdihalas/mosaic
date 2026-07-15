// Package policy defines deterministic policy results.
package policy

import (
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"sort"
)

type RuleType string

const (
	Require RuleType = "require"
	Deny    RuleType = "deny"
	Warn    RuleType = "warn"
)

type Result struct {
	Policy         string               `json:"policy"`
	Rule           RuleType             `json:"rule"`
	Severity       diagnostics.Severity `json:"severity"`
	ResourceID     graph.ResourceID     `json:"resourceId"`
	Message        string               `json:"message"`
	PolicySource   diagnostics.Span     `json:"policySource"`
	ResourceSource diagnostics.Span     `json:"resourceSource"`
	Field          graph.FieldPath      `json:"field,omitempty"`
}
type Report struct {
	Results []Result `json:"results"`
}

func (r *Report) Sort() {
	sort.SliceStable(r.Results, func(i, j int) bool {
		a, b := r.Results[i], r.Results[j]
		if a.Policy != b.Policy {
			return a.Policy < b.Policy
		}
		if a.ResourceID != b.ResourceID {
			return a.ResourceID < b.ResourceID
		}
		return a.PolicySource.Start.Offset < b.PolicySource.Start.Offset
	})
}
func (r Report) HasErrors() bool {
	for _, x := range r.Results {
		if x.Severity == diagnostics.SeverityError {
			return true
		}
	}
	return false
}
