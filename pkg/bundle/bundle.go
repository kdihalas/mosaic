// Package bundle creates and verifies deterministic Mosaic deployment bundles.
package bundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kdihalas/mosaic/pkg/capability"
	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/module"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/project"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const FormatVersion = "v1alpha1"

type Artifact struct {
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
}
type Manifest struct {
	FormatVersion   string     `json:"formatVersion"`
	Environment     string     `json:"environment"`
	CompilerVersion string     `json:"compilerVersion"`
	SourceDigest    string     `json:"sourceDigest"`
	GraphDigest     string     `json:"graphDigest"`
	BundleDigest    string     `json:"bundleDigest"`
	Artifacts       []Artifact `json:"artifacts"`
}
type Bundle struct {
	Manifest Manifest          `json:"manifest"`
	Files    map[string][]byte `json:"-"`
	Graph    *graph.Graph      `json:"-"`
	Recipe   *Recipe           `json:"-"`
}
type BuildOptions struct{}

type SchemaDocument struct {
	FormatVersion string           `json:"formatVersion"`
	Resources     []SchemaResource `json:"resources"`
	Exports       []SchemaExport   `json:"exports,omitempty"`
}

type SchemaResource struct {
	ID   graph.ResourceID `json:"id"`
	Type graph.TypeName   `json:"type"`
}

type SchemaExport struct {
	ID       graph.ResourceID `json:"id"`
	Optional bool             `json:"optional,omitempty"`
	Present  bool             `json:"present"`
}

type ExtensionPointsDocument struct {
	FormatVersion string                    `json:"formatVersion"`
	Resources     []ResourceExtensionPoints `json:"resources"`
}

type ResourceExtensionPoints struct {
	ID         graph.ResourceID  `json:"id"`
	Extensions []graph.FieldPath `json:"extensions,omitempty"`
	Protected  []graph.FieldPath `json:"protected,omitempty"`
}

// Recipe is the path-independent compiler input embedded in composable
// bundles. It contains no registry references or filesystem paths.
type Recipe struct {
	Project  RecipeProject   `json:"project"`
	Packages []RecipePackage `json:"packages,omitempty"`
}

type RecipeProject struct {
	Name   string         `json:"name"`
	Config project.Config `json:"config"`
	Files  []RecipeFile   `json:"files"`
}

type RecipePackage struct {
	Identity     string                       `json:"identity"`
	Version      string                       `json:"version"`
	Aliases      []string                     `json:"aliases,omitempty"`
	Files        []RecipeFile                 `json:"files"`
	Manifest     mosaicpackage.Manifest       `json:"manifest"`
	Dependencies []compiler.PackageDependency `json:"dependencies,omitempty"`
}

type RecipeFile struct {
	Name    string `json:"name"`
	Content []byte `json:"content"`
}

func Build(r *compiler.Result, a renderer.ArtifactSet, _ BuildOptions) (*Bundle, error) {
	g, err := r.Graph.CanonicalJSON()
	if err != nil {
		return nil, err
	}
	prov, err := json.MarshalIndent(r.Provenance.Events(), "", "  ")
	if err != nil {
		return nil, err
	}
	prov = append(prov, '\n')
	pol, err := json.MarshalIndent(r.PolicyReport, "", "  ")
	if err != nil {
		return nil, err
	}
	pol = append(pol, '\n')
	schema, extensions, err := graphMetadata(r.Graph, r.Instances, r.Capabilities)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"graph.json": append(g, '\n'), "schema.json": schema,
		"extension-points.json": extensions, "provenance.json": prov, "policy-report.json": pol,
	}
	recipe, err := recipeFromInput(r.BuildInput)
	if err != nil {
		return nil, err
	}
	if recipe != nil {
		recipeBytes, err := json.MarshalIndent(recipe, "", "  ")
		if err != nil {
			return nil, err
		}
		files["build-recipe.json"] = append(recipeBytes, '\n')
	}
	for k, v := range a.Files {
		if !safe(k) {
			return nil, fmt.Errorf("unsafe artifact path %q", k)
		}
		files[k] = append([]byte(nil), v...)
	}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	arts := make([]Artifact, len(names))
	for i, n := range names {
		arts[i] = Artifact{Name: n, MediaType: media(n), Digest: digest(files[n])}
	}
	src := r.Metadata.SourceDigest
	if src == "" {
		src = digest(nil)
	}
	m := Manifest{FormatVersion: FormatVersion, Environment: r.Environment, CompilerVersion: r.Metadata.CompilerVersion, SourceDigest: src, GraphDigest: digest(g), Artifacts: arts}
	seed, _ := json.Marshal(m)
	m.BundleDigest = digest(seed)
	b := &Bundle{Manifest: m, Files: files, Graph: r.Graph.Snapshot(), Recipe: recipe}
	manifest, _ := json.MarshalIndent(m, "", "  ")
	b.Files["bundle.json"] = append(manifest, '\n')
	return b, nil
}

func recipeFromInput(input compiler.Input) (*Recipe, error) {
	if input.RootProject == nil {
		return nil, nil
	}
	recipe := &Recipe{Project: RecipeProject{Name: input.RootProject.Name, Config: input.RootProject.Config, Files: recipeFiles(input.RootProject.Files)}}
	for _, pkg := range input.Packages {
		recipe.Packages = append(recipe.Packages, RecipePackage{
			Identity: pkg.Identity.String(), Version: pkg.Version.String(), Aliases: append([]string(nil), pkg.Aliases...),
			Files: recipeFiles(pkg.Files), Manifest: pkg.Manifest, Dependencies: append([]compiler.PackageDependency(nil), pkg.Dependencies...),
		})
	}
	sort.Slice(recipe.Packages, func(i, j int) bool {
		if recipe.Packages[i].Identity != recipe.Packages[j].Identity {
			return recipe.Packages[i].Identity < recipe.Packages[j].Identity
		}
		return recipe.Packages[i].Version < recipe.Packages[j].Version
	})
	return recipe, nil
}

func recipeFiles(files []source.File) []RecipeFile {
	out := make([]RecipeFile, len(files))
	for i, file := range files {
		out[i] = RecipeFile{Name: file.Name, Content: append([]byte(nil), file.Content...)}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CompilerInput reconstructs an in-memory compiler input from a verified
// composable bundle recipe.
func (r Recipe) CompilerInput(environment string) (compiler.Input, error) {
	root := project.New(".", r.Project.Name, sourceFiles(r.Project.Files))
	root.Config = r.Project.Config
	input := compiler.Input{RootProject: root, Environment: environment}
	for _, pkg := range r.Packages {
		identity, err := mosaicpackage.ParseIdentity(pkg.Identity)
		if err != nil {
			return compiler.Input{}, err
		}
		version, err := mosaicpackage.ParseVersion(pkg.Version)
		if err != nil {
			return compiler.Input{}, err
		}
		input.Packages = append(input.Packages, compiler.CompilationPackage{
			Identity: identity, Version: version, Aliases: append([]string(nil), pkg.Aliases...),
			Files: sourceFiles(pkg.Files), Manifest: pkg.Manifest, Exports: pkg.Manifest.Exports,
			Dependencies: append([]compiler.PackageDependency(nil), pkg.Dependencies...),
		})
	}
	return input, nil
}

func sourceFiles(files []RecipeFile) []source.File {
	out := make([]source.File, len(files))
	for i, file := range files {
		out[i] = source.NewFile(file.Name, file.Content)
	}
	return out
}

func graphMetadata(g *graph.Graph, instances []module.Instance, capabilities []capability.Instance) ([]byte, []byte, error) {
	schema := SchemaDocument{FormatVersion: FormatVersion}
	extensions := ExtensionPointsDocument{FormatVersion: FormatVersion}
	for _, resource := range g.List() {
		schema.Resources = append(schema.Resources, SchemaResource{ID: resource.ID, Type: resource.Type})
		extensions.Resources = append(extensions.Resources, ResourceExtensionPoints{
			ID: resource.ID, Extensions: resource.Metadata.Extensions, Protected: resource.Metadata.Protected,
		})
	}
	for _, instance := range instances {
		for _, exported := range instance.Exports {
			schema.Exports = append(schema.Exports, SchemaExport{ID: exported.ResourceID, Optional: exported.Optional, Present: exported.Present})
		}
	}
	for _, instance := range capabilities {
		for _, exported := range instance.Exports {
			schema.Exports = append(schema.Exports, SchemaExport{ID: exported.ResourceID, Optional: exported.Optional, Present: exported.Present})
		}
	}
	sort.Slice(schema.Exports, func(i, j int) bool { return schema.Exports[i].ID < schema.Exports[j].ID })
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	extensionBytes, err := json.MarshalIndent(extensions, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return append(schemaBytes, '\n'), append(extensionBytes, '\n'), nil
}
func WriteDirectory(ctx context.Context, b *Bundle, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".mosaic-bundle-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	names := make([]string, 0, len(b.Files))
	for n := range b.Files {
		if !safe(n) {
			return fmt.Errorf("unsafe artifact path %q", n)
		}
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		target := filepath.Join(tmp, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, b.Files[n], 0o644); err != nil {
			return err
		}
	}
	old := path + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(path); err == nil {
		if err = os.Rename(path, old); err != nil {
			return err
		}
	}
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Rename(old, path)
		return err
	}
	return os.RemoveAll(old)
}
func ReadDirectory(ctx context.Context, path string) (*Bundle, diagnostics.List) {
	select {
	case <-ctx.Done():
		return nil, one(ctx.Err().Error(), path)
	default:
	}
	raw, err := os.ReadFile(filepath.Join(path, "bundle.json"))
	if err != nil {
		return nil, one(err.Error(), path)
	}
	var m Manifest
	if err = json.Unmarshal(raw, &m); err != nil {
		return nil, one(err.Error(), path)
	}
	if m.FormatVersion != FormatVersion {
		return nil, one("unsupported bundle format "+m.FormatVersion, path)
	}
	wantBundle := m.BundleDigest
	m.BundleDigest = ""
	seed, _ := json.Marshal(m)
	m.BundleDigest = wantBundle
	if digest(seed) != wantBundle {
		return nil, one("bundle digest mismatch", "bundle.json")
	}
	files := map[string][]byte{"bundle.json": raw}
	for _, a := range m.Artifacts {
		if !safe(a.Name) {
			return nil, one("unsafe artifact path", a.Name)
		}
		v, e := os.ReadFile(filepath.Join(path, filepath.FromSlash(a.Name)))
		if e != nil {
			return nil, one(e.Error(), a.Name)
		}
		if digest(v) != a.Digest {
			return nil, one("artifact digest mismatch", a.Name)
		}
		files[a.Name] = v
	}
	for _, required := range []string{"graph.json", "schema.json", "extension-points.json"} {
		if _, ok := files[required]; !ok {
			return nil, one("required bundle artifact is missing", required)
		}
	}
	var recipe *Recipe
	if rawRecipe, ok := files["build-recipe.json"]; ok {
		var parsed Recipe
		if err := json.Unmarshal(rawRecipe, &parsed); err != nil {
			return nil, one(err.Error(), "build-recipe.json")
		}
		recipe = &parsed
	}
	g, err := graph.DecodeCanonical(files["graph.json"])
	if err != nil {
		return nil, one(err.Error(), "graph.json")
	}
	if digest(bytes.TrimSpace(files["graph.json"])) != m.GraphDigest {
		return nil, one("graph digest mismatch", "graph.json")
	}
	return &Bundle{Manifest: m, Files: files, Graph: g.Snapshot(), Recipe: recipe}, nil
}
func safe(n string) bool {
	return n != "" && !filepath.IsAbs(n) && !strings.Contains(filepath.ToSlash(n), "../") && filepath.Clean(n) != ".."
}
func digest(b []byte) string { x := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(x[:]) }
func media(n string) string {
	switch filepath.Ext(n) {
	case ".yaml", ".yml":
		return "application/yaml"
	default:
		return "application/json"
	}
}
func one(msg, name string) diagnostics.List {
	return diagnostics.List{{Code: "BND001", Severity: diagnostics.SeverityError, Message: msg, Span: diagnostics.Span{SourceName: name}}}
}
