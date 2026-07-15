package dependency

import "sort"

type PackageID string

type Edge struct {
	From       PackageID `json:"from"`
	To         PackageID `json:"to"`
	Constraint string    `json:"constraint"`
	Alias      string    `json:"alias,omitempty"`
}

type Graph struct {
	Root  string      `json:"root"`
	Nodes []PackageID `json:"nodes"`
	Edges []Edge      `json:"edges"`
}

func (g Graph) Sorted() Graph {
	out := g
	out.Nodes = append([]PackageID(nil), g.Nodes...)
	out.Edges = append([]Edge(nil), g.Edges...)
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i] < out.Nodes[j] })
	sort.Slice(out.Edges, func(i, j int) bool {
		if out.Edges[i].From != out.Edges[j].From {
			return out.Edges[i].From < out.Edges[j].From
		}
		if out.Edges[i].To != out.Edges[j].To {
			return out.Edges[i].To < out.Edges[j].To
		}
		return out.Edges[i].Alias < out.Edges[j].Alias
	})
	return out
}
