// Package testing runs deterministic Mosaic configuration tests.
package testing

import (
	"context"
	"fmt"
	"sort"

	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/semantic"
	"github.com/kdihalas/mosaic/pkg/syntax/ast"
	"github.com/kdihalas/mosaic/pkg/value"
)

type CaseResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}
type Report struct {
	Cases  []CaseResult `json:"cases"`
	Passed int          `json:"passed"`
	Failed int          `json:"failed"`
}

func Run(ctx context.Context, p *project.Project, c *compiler.Compiler) (Report, diagnostics.List) {
	a, ds := c.Analyze(ctx, p)
	if ds.HasErrors() {
		return Report{}, ds
	}
	var tests []*ast.TestDeclaration
	for _, f := range a.Files {
		for _, d := range f.Declarations {
			if t, ok := d.(*ast.TestDeclaration); ok {
				tests = append(tests, t)
			}
		}
	}
	sort.Slice(tests, func(i, j int) bool { return tests[i].Name < tests[j].Name })
	var report Report
	for _, t := range tests {
		cr := CaseResult{Name: t.Name}
		env := ""
		for _, s := range t.Body {
			if o, ok := s.(*ast.OperationStatement); ok && o.Operation == "build" {
				env = o.Name
			}
		}
		if env == "" {
			cr.Message = "test has no build statement"
			report.Failed++
			report.Cases = append(report.Cases, cr)
			continue
		}
		r, cd := c.Compile(ctx, p, compiler.Options{Environment: env})
		if cd.HasErrors() {
			cr.Message = cd[0].Message
			report.Failed++
			report.Cases = append(report.Cases, cr)
			continue
		}
		passed := true
		for _, s := range t.Body {
			o, ok := s.(*ast.OperationStatement)
			if !ok || o.Operation != "assert" || o.Value == nil {
				continue
			}
			v, e := semantic.Evaluate(o.Value, semantic.Context{ResolvePath: func(path []string) (value.Value, bool) {
				if len(path) < 3 {
					return value.Value{}, false
				}
				id := graph.ResourceID("application." + path[0] + "." + path[1] + "." + path[2])
				if len(path) == 3 {
					if q, yes := r.Graph.Get(id); yes {
						return q.Fields, true
					}
					return value.Null(), true
				}
				return r.Graph.ReadField(id, graph.FieldPath(path[3:]))
			}})
			if e != nil {
				passed = false
				cr.Message = e.Error()
				break
			}
			b, ok := v.BoolValue()
			if !ok || !b {
				passed = false
				cr.Message = fmt.Sprintf("assertion failed at %s:%d", s.Span().SourceName, s.Span().Start.Line)
				break
			}
		}
		cr.Passed = passed
		if passed {
			report.Passed++
		} else {
			report.Failed++
		}
		report.Cases = append(report.Cases, cr)
	}
	return report, ds
}
