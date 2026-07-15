package dependency

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/Masterminds/semver/v3"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/project"
)

type SourceReference struct {
	Source        string `json:"source,omitempty"`
	Path          string `json:"path,omitempty"`
	DeclaringRoot string `json:"-"`
}

func (r SourceReference) String() string {
	if r.Path != "" {
		return "path:" + r.Path
	}
	return r.Source
}

type ExactPackageReference struct {
	SourceReference
	Identity mosaicpackage.Identity
	Version  mosaicpackage.Version
	Digest   string
}

type VersionMetadata struct {
	Version           mosaicpackage.Version
	Manifest          mosaicpackage.Manifest
	Source            string
	Root              string
	ManifestDigest    string
	ContentDigest     string
	ArchiveDigest     string
	OCIManifestDigest string
}

type SourceResolver interface {
	Supports(SourceReference) bool
	ListVersions(context.Context, SourceReference) ([]VersionMetadata, diagnostics.List)
	FetchManifest(context.Context, ExactPackageReference) (*mosaicpackage.Manifest, diagnostics.List)
	FetchPackage(context.Context, ExactPackageReference) (*mosaicpackage.Package, diagnostics.List)
}

type ResolveOptions struct {
	Offline           bool
	PreferLocked      bool
	IncludePrerelease bool
	Limits            mosaicpackage.Limits
}

type ResolveInput struct {
	Project      project.Config
	ProjectRoot  string
	ExistingLock *lockfile.File
	Sources      []SourceResolver
	Options      ResolveOptions
}

type ResolvedPackage struct {
	Identity          mosaicpackage.Identity `json:"identity"`
	Version           mosaicpackage.Version  `json:"version"`
	Source            string                 `json:"source"`
	DeclaredSource    string                 `json:"declaredSource,omitempty"`
	Aliases           []string               `json:"aliases,omitempty"`
	Manifest          mosaicpackage.Manifest `json:"manifest"`
	Root              string                 `json:"root,omitempty"`
	ManifestDigest    string                 `json:"manifestDigest"`
	ContentDigest     string                 `json:"contentDigest"`
	ArchiveDigest     string                 `json:"archiveDigest,omitempty"`
	OCIManifestDigest string                 `json:"ociManifestDigest,omitempty"`
	Dependencies      []PackageID            `json:"dependencies,omitempty"`
	Replacement       bool                   `json:"replacement,omitempty"`
}

func (p ResolvedPackage) ID() PackageID {
	return PackageID(p.Identity.String() + "@" + p.Version.String())
}

type Resolution struct {
	Root     string            `json:"root"`
	Packages []ResolvedPackage `json:"packages"`
	Graph    Graph             `json:"graph"`
}

func (r Resolution) Lock(cfg project.Config) lockfile.File {
	f := lockfile.File{FormatVersion: lockfile.FormatVersion, Project: cfg.Name, DependencyDigest: project.DependencyDigest(cfg)}
	for _, pkg := range r.Packages {
		deps := make([]string, len(pkg.Dependencies))
		for i := range pkg.Dependencies {
			deps[i] = string(pkg.Dependencies[i])
		}
		f.Packages = append(f.Packages, lockfile.Package{Identity: pkg.Identity.String(), Version: pkg.Version.String(), Source: pkg.Source, DeclaredSource: pkg.DeclaredSource, Aliases: append([]string(nil), pkg.Aliases...), ManifestDigest: pkg.ManifestDigest, ContentDigest: pkg.ContentDigest, ArchiveDigest: pkg.ArchiveDigest, OCIManifestDigest: pkg.OCIManifestDigest, Dependencies: deps, Replacement: pkg.Replacement})
	}
	return f.Sorted()
}

type Resolver struct{ sources []SourceResolver }

func NewResolver(sources ...SourceResolver) *Resolver {
	return &Resolver{sources: append([]SourceResolver(nil), sources...)}
}

type requirement struct {
	constraint    Constraint
	parent, alias string
}
type sourceEntry struct {
	ref         SourceReference
	declared    string
	effective   string
	candidates  []VersionMetadata
	replacement bool
}
type solveState struct {
	requirements map[string][]requirement
	sources      map[string]sourceEntry
	selected     map[string]ResolvedPackage
	edges        []Edge
}

func (r *Resolver) Resolve(ctx context.Context, input ResolveInput) (*Resolution, diagnostics.List) {
	if len(input.Sources) > 0 {
		r = NewResolver(input.Sources...)
	}
	limits := input.Options.Limits.WithDefaults()
	state := solveState{requirements: map[string][]requirement{}, sources: map[string]sourceEntry{}, selected: map[string]ResolvedPackage{}}
	aliases := make([]string, 0, len(input.Project.Dependencies))
	for alias := range input.Project.Dependencies {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		if err := validateAlias(alias); err != nil {
			return nil, depDiagnostic("DEP001", err.Error())
		}
		dep := input.Project.Dependencies[alias]
		if ds := r.addDeclaration(ctx, &state, input, "", alias, dep, input.ProjectRoot, limits); ds.HasErrors() {
			return nil, ds
		}
	}
	final, ds := r.solve(ctx, input, state, limits)
	if ds.HasErrors() {
		return nil, ds
	}
	packages := make([]ResolvedPackage, 0, len(final.selected))
	for _, pkg := range final.selected {
		for i, dependencyID := range pkg.Dependencies {
			identity := string(dependencyID)
			if selected, ok := final.selected[identity]; ok {
				pkg.Dependencies[i] = selected.ID()
			}
		}
		pkg.Aliases = sortedStrings(pkg.Aliases)
		pkg.Dependencies = sortedIDs(pkg.Dependencies)
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Identity < packages[j].Identity })
	graph := Graph{Root: input.Project.Name, Edges: final.edges}
	for _, pkg := range packages {
		graph.Nodes = append(graph.Nodes, pkg.ID())
	}
	graph = graph.Sorted()
	if cycle := findCycle(graph); len(cycle) > 0 {
		return nil, depDiagnostic("DEP015", "circular package dependency: "+strings.Join(cycle, " -> "))
	}
	return &Resolution{Root: input.Project.Name, Packages: packages, Graph: graph}, nil
}

func (r *Resolver) addDeclaration(ctx context.Context, state *solveState, input ResolveInput, parent, alias string, declaration mosaicpackage.Dependency, declaringRoot string, limits mosaicpackage.Limits) diagnostics.List {
	ref := SourceReference{Source: declaration.Source, Path: declaration.Path, DeclaringRoot: declaringRoot}
	versions, ds := r.list(ctx, ref)
	if ds.HasErrors() {
		return ds
	}
	if len(versions) == 0 {
		return depDiagnostic("DEP010", "package not found at "+ref.String())
	}
	if len(versions) > limits.MaxAvailableVersions {
		return depDiagnostic("DEP010", "available version limit exceeded for "+ref.String())
	}
	identity := versions[0].Manifest.Name
	for _, candidate := range versions {
		if candidate.Manifest.Name != identity {
			return depDiagnostic("DEP020", "source contains conflicting package identities")
		}
	}
	if replacement, ok := input.Project.Replace[identity]; ok {
		ref = SourceReference{Source: replacement.Source, Path: replacement.Path, DeclaringRoot: input.ProjectRoot}
		versions, ds = r.list(ctx, ref)
		if ds.HasErrors() {
			return ds
		}
		if len(versions) == 0 || versions[0].Manifest.Name != identity {
			return depDiagnostic("DEP020", fmt.Sprintf("replacement identity mismatch for %s", identity))
		}
		for i := range versions {
			versions[i].Source = canonicalSource(ref, versions[i])
		}
	}
	constraintRaw := declaration.Version
	if declaration.Path != "" {
		constraintRaw = "*"
	}
	constraint, err := ParseConstraint(constraintRaw)
	if err != nil {
		return depDiagnostic("DEP002", err.Error())
	}
	effective := canonicalSource(ref, versions[0])
	entry, exists := state.sources[identity]
	if exists && entry.effective != effective {
		return depDiagnostic("DEP020", fmt.Sprintf("package %s is claimed by both %s and %s", identity, entry.effective, effective))
	}
	if !exists {
		declared := declaration.Source
		if declaration.Path != "" {
			declared = "path:" + declaration.Path
		}
		entry = sourceEntry{ref: ref, declared: declared, effective: effective, candidates: versions, replacement: ref.String() != SourceReference{Source: declaration.Source, Path: declaration.Path}.String()}
		state.sources[identity] = entry
	}
	state.requirements[identity] = append(state.requirements[identity], requirement{constraint: constraint, parent: parent, alias: alias})
	from := parent
	if from == "" {
		from = input.Project.Name
	}
	state.edges = append(state.edges, Edge{From: PackageID(from), To: PackageID(identity), Constraint: constraint.String(), Alias: alias})
	return nil
}

func (r *Resolver) solve(ctx context.Context, input ResolveInput, state solveState, limits mosaicpackage.Limits) (solveState, diagnostics.List) {
	if err := ctx.Err(); err != nil {
		return state, depDiagnostic("DEP010", err.Error())
	}
	if len(state.selected) > limits.MaxDependencies {
		return state, depDiagnostic("DEP010", "dependency count limit exceeded")
	}
	for identity, selected := range state.selected {
		if !matchesAll(selected.Version.Semver(), state.requirements[identity], input.Options.IncludePrerelease) {
			return state, conflictDiagnostic(identity, state.requirements[identity])
		}
	}
	identity := ""
	for candidate := range state.requirements {
		if _, ok := state.selected[candidate]; !ok && (identity == "" || candidate < identity) {
			identity = candidate
		}
	}
	if identity == "" {
		return state, nil
	}
	entry := state.sources[identity]
	candidates := orderCandidates(entry.candidates, identity, input.ExistingLock, input.Options)
	var last diagnostics.List
	for _, candidate := range candidates {
		version := candidate.Version.Semver()
		if !matchesAll(version, state.requirements[identity], input.Options.IncludePrerelease) {
			continue
		}
		next := cloneState(state)
		aliases := directAliases(next.requirements[identity])
		pkg := ResolvedPackage{Identity: mosaicpackage.Identity(identity), Version: candidate.Version, Source: canonicalSource(entry.ref, candidate), DeclaredSource: entry.declared, Aliases: aliases, Manifest: candidate.Manifest, Root: candidate.Root, ManifestDigest: candidate.ManifestDigest, ContentDigest: candidate.ContentDigest, ArchiveDigest: candidate.ArchiveDigest, OCIManifestDigest: candidate.OCIManifestDigest, Replacement: entry.replacement}
		if !entry.replacement || pkg.DeclaredSource == pkg.Source {
			pkg.DeclaredSource = ""
		}
		next.selected[identity] = pkg
		depAliases := make([]string, 0, len(candidate.Manifest.Dependencies))
		for alias := range candidate.Manifest.Dependencies {
			depAliases = append(depAliases, alias)
		}
		sort.Strings(depAliases)
		failed := false
		for _, alias := range depAliases {
			if ds := r.addDeclaration(ctx, &next, input, string(pkg.ID()), alias, candidate.Manifest.Dependencies[alias], candidate.Root, limits); ds.HasErrors() {
				last = ds
				failed = true
				break
			}
			childIdentity := identityForAlias(next, string(pkg.ID()), alias)
			if childIdentity != "" {
				p := next.selected[identity]
				p.Dependencies = append(p.Dependencies, PackageID(childIdentity))
				next.selected[identity] = p
			}
		}
		if failed {
			continue
		}
		resolved, ds := r.solve(ctx, input, next, limits)
		if !ds.HasErrors() {
			for i := range resolved.edges {
				if resolved.edges[i].To == PackageID(identity) {
					resolved.edges[i].To = pkg.ID()
				}
			}
			return resolved, nil
		}
		last = ds
	}
	if last.HasErrors() {
		return state, last
	}
	return state, conflictDiagnostic(identity, state.requirements[identity])
}

func (r *Resolver) list(ctx context.Context, ref SourceReference) ([]VersionMetadata, diagnostics.List) {
	for _, source := range r.sources {
		if source.Supports(ref) {
			return source.ListVersions(ctx, ref)
		}
	}
	if ref.Source != "" {
		return nil, depDiagnostic("DEP041", "package source is unavailable: "+ref.Source)
	}
	return nil, depDiagnostic("DEP010", "no resolver supports "+ref.String())
}

func orderCandidates(in []VersionMetadata, identity string, existing *lockfile.File, options ResolveOptions) []VersionMetadata {
	out := append([]VersionMetadata(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Version.Semver(), out[j].Version.Semver()
		if (a.Prerelease() == "") != (b.Prerelease() == "") {
			return a.Prerelease() == ""
		}
		return a.GreaterThan(b)
	})
	if options.PreferLocked && existing != nil {
		if locked, ok := existing.Lookup(identity); ok {
			for i := range out {
				if out[i].Version.String() == locked.Version {
					candidate := out[i]
					copy(out[1:i+1], out[:i])
					out[0] = candidate
					break
				}
			}
		}
	}
	return out
}

func matchesAll(version *semver.Version, requirements []requirement, includePrerelease bool) bool {
	for _, req := range requirements {
		if !req.constraint.Check(version, includePrerelease) {
			return false
		}
	}
	return true
}
func canonicalSource(ref SourceReference, metadata VersionMetadata) string {
	if metadata.Source != "" {
		return metadata.Source
	}
	return ref.String()
}
func directAliases(reqs []requirement) []string {
	var out []string
	for _, req := range reqs {
		if req.parent == "" {
			out = append(out, req.alias)
		}
	}
	return sortedStrings(out)
}
func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
func sortedIDs(in []PackageID) []PackageID {
	out := append([]PackageID(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
func identityForAlias(state solveState, parent, alias string) string {
	for identity, reqs := range state.requirements {
		for _, req := range reqs {
			if req.parent == parent && req.alias == alias {
				return identity
			}
		}
	}
	return ""
}
func cloneState(state solveState) solveState {
	out := solveState{requirements: map[string][]requirement{}, sources: map[string]sourceEntry{}, selected: map[string]ResolvedPackage{}, edges: append([]Edge(nil), state.edges...)}
	for k, v := range state.requirements {
		out.requirements[k] = append([]requirement(nil), v...)
	}
	for k, v := range state.sources {
		v.candidates = append([]VersionMetadata(nil), v.candidates...)
		out.sources[k] = v
	}
	for k, v := range state.selected {
		v.Aliases = append([]string(nil), v.Aliases...)
		v.Dependencies = append([]PackageID(nil), v.Dependencies...)
		out.selected[k] = v
	}
	return out
}
func validateAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("dependency alias is empty")
	}
	for i, r := range alias {
		if i == 0 && r != '_' && !unicode.IsLetter(r) {
			return fmt.Errorf("invalid dependency alias %q", alias)
		}
		if i > 0 && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && !unicode.Is(unicode.Pc, r) {
			return fmt.Errorf("invalid dependency alias %q", alias)
		}
	}
	return nil
}
func depDiagnostic(code, message string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: message, Suggestion: "run `mosaic deps resolve`"}}
}
func conflictDiagnostic(identity string, reqs []requirement) diagnostics.List {
	notes := make([]string, 0, len(reqs))
	for _, req := range reqs {
		parent := req.parent
		if parent == "" {
			parent = "root"
		}
		notes = append(notes, fmt.Sprintf("required by %s: %s", parent, req.constraint.String()))
	}
	sort.Strings(notes)
	return diagnostics.List{{Code: "DEP012", Severity: diagnostics.SeverityError, Message: "incompatible dependency constraints for " + identity, Notes: notes}}
}

func findCycle(graph Graph) []string {
	adj := map[string][]string{}
	for _, edge := range graph.Edges {
		adj[string(edge.From)] = append(adj[string(edge.From)], string(edge.To))
	}
	for k := range adj {
		sort.Strings(adj[k])
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var stack []string
	var visit func(string) []string
	visit = func(node string) []string {
		if visiting[node] {
			for i, n := range stack {
				if n == node {
					return append(append([]string(nil), stack[i:]...), node)
				}
			}
		}
		if done[node] {
			return nil
		}
		visiting[node] = true
		stack = append(stack, node)
		for _, child := range adj[node] {
			if c := visit(child); len(c) > 0 {
				return c
			}
		}
		stack = stack[:len(stack)-1]
		visiting[node] = false
		done[node] = true
		return nil
	}
	keys := make([]string, 0, len(adj))
	for k := range adj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if c := visit(k); len(c) > 0 {
			return c
		}
	}
	return nil
}
