package mosaicpackage

import "sort"

// ExportSet is the explicit public surface of a package.
type ExportSet struct {
	Modules      []string `toml:"modules,omitempty" json:"modules,omitempty"`
	Types        []string `toml:"types,omitempty" json:"types,omitempty"`
	Enums        []string `toml:"enums,omitempty" json:"enums,omitempty"`
	Variants     []string `toml:"variants,omitempty" json:"variants,omitempty"`
	Transforms   []string `toml:"transforms,omitempty" json:"transforms,omitempty"`
	Policies     []string `toml:"policies,omitempty" json:"policies,omitempty"`
	Environments []string `toml:"environments,omitempty" json:"environments,omitempty"`
	Capabilities []string `toml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Schemas      []string `toml:"schemas,omitempty" json:"schemas,omitempty"`
	Tests        []string `toml:"tests,omitempty" json:"tests,omitempty"`
}

func (e ExportSet) Sorted() ExportSet {
	return ExportSet{
		Modules: sortedCopy(e.Modules), Types: sortedCopy(e.Types), Enums: sortedCopy(e.Enums),
		Variants: sortedCopy(e.Variants), Transforms: sortedCopy(e.Transforms), Policies: sortedCopy(e.Policies), Environments: sortedCopy(e.Environments),
		Capabilities: sortedCopy(e.Capabilities), Schemas: sortedCopy(e.Schemas), Tests: sortedCopy(e.Tests),
	}
}

// Categories returns a stable category-to-name view.
func (e ExportSet) Categories() map[string][]string {
	return map[string][]string{
		"module": e.Modules, "type": e.Types, "enum": e.Enums, "variant": e.Variants,
		"transform": e.Transforms, "policy": e.Policies, "environment": e.Environments, "capability": e.Capabilities,
		"schema": e.Schemas, "test": e.Tests,
	}
}

// Contains reports whether a named symbol is exported in the category.
func (e ExportSet) Contains(category, name string) bool {
	for _, n := range e.Categories()[category] {
		if n == name {
			return true
		}
	}
	return false
}

// Names returns all exports sorted by category then name.
func (e ExportSet) Names() []string {
	var out []string
	for category, names := range e.Categories() {
		for _, name := range names {
			out = append(out, category+"."+name)
		}
	}
	sort.Strings(out)
	return out
}
