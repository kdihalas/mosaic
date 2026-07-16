// Package testing runs deterministic Mosaic configuration tests.
package testing

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/module"
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
	return RunInput(ctx, compiler.Input{RootProject: p}, c)
}

// RunInput executes tests with explicit restored compilation packages.
func RunInput(ctx context.Context, input compiler.Input, c *compiler.Compiler) (Report, diagnostics.List) {
	a, ds := c.AnalyzeInput(ctx, input)
	if ds.HasErrors() {
		return Report{}, ds
	}
	var tests []*ast.TestDeclaration
	for _, f := range a.Files {
		// Dependency package tests are validated by their owning package and do
		// not become root project tests merely because package sources are loaded.
		if strings.HasPrefix(f.Name, "package/") {
			continue
		}
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
		compileInput := input
		compileInput.Environment = env
		r, cd := c.CompileInput(ctx, compileInput)
		if cd.HasErrors() {
			cr.Message = cd[0].Message
			report.Failed++
			report.Cases = append(report.Cases, cr)
			continue
		}
		passed := true
		exports := map[graph.ResourceID]module.Export{}
		for _, instance := range r.Instances {
			for _, exported := range instance.Exports {
				exports[exported.ResourceID] = exported
			}
		}
		for _, s := range t.Body {
			o, ok := s.(*ast.OperationStatement)
			if !ok || o.Operation != "assert" || o.Value == nil {
				continue
			}
			v, e := semantic.Evaluate(o.Value, semantic.Context{StrictPaths: true, ResolvePath: func(path []string) (value.Value, bool) {
				if len(path) < 3 {
					return value.Value{}, false
				}
				id := graph.ResourceID("application." + path[0] + "." + path[1] + "." + path[2])
				if exported, known := exports[id]; known && exported.Optional {
					return value.Value{}, false
				}
				if resource, found := r.Graph.Get(id); found && resource.Metadata.Module != "" && !resource.Metadata.Exported {
					return value.Value{}, false
				}
				if len(path) == 3 {
					if q, yes := r.Graph.Get(id); yes {
						return q.Fields, true
					}
					return value.Null(), true
				}
				return r.Graph.ReadField(id, graph.FieldPath(path[3:]))
			}, PresentPath: func(path []string) (bool, bool) {
				if len(path) != 3 {
					return false, false
				}
				id := graph.ResourceID("application." + path[0] + "." + path[1] + "." + path[2])
				exported, ok := exports[id]
				if !ok || !exported.Optional {
					return false, false
				}
				return exported.Present, true
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
