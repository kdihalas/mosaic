// Package bundle creates and verifies deterministic Mosaic deployment bundles.
package bundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kdihalas/mosaic/pkg/compiler"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/renderer"
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
}
type BuildOptions struct{}

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
	files := map[string][]byte{"graph.json": append(g, '\n'), "provenance.json": prov, "policy-report.json": pol}
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
	b := &Bundle{Manifest: m, Files: files, Graph: r.Graph.Snapshot()}
	manifest, _ := json.MarshalIndent(m, "", "  ")
	b.Files["bundle.json"] = append(manifest, '\n')
	return b, nil
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
	g, err := graph.DecodeCanonical(files["graph.json"])
	if err != nil {
		return nil, one(err.Error(), "graph.json")
	}
	if digest(bytes.TrimSpace(files["graph.json"])) != m.GraphDigest {
		return nil, one("graph digest mismatch", "graph.json")
	}
	return &Bundle{Manifest: m, Files: files, Graph: g.Snapshot()}, nil
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
