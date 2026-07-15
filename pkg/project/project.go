// Package project loads deterministic collections of Mosaic source files.
package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Name                           string                              `toml:"name" json:"name"`
	LanguageVersion                string                              `toml:"language_version" json:"languageVersion"`
	Sources                        []string                            `toml:"sources" json:"sources"`
	Exclude                        []string                            `toml:"exclude" json:"exclude"`
	DefaultOutput                  string                              `toml:"default_output" json:"defaultOutput"`
	AllowExternalLocalDependencies bool                                `toml:"allow_external_local_dependencies,omitempty" json:"allowExternalLocalDependencies,omitempty"`
	Dependencies                   map[string]mosaicpackage.Dependency `toml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Replace                        map[string]mosaicpackage.Dependency `toml:"replace,omitempty" json:"replace,omitempty"`
}
type Project struct {
	Root   string
	Name   string
	Files  []source.File
	Config Config
}
type LoadOptions struct{ ConfigFile string }

// LoadManifest reads only mosaic.toml and applies project defaults.
func LoadManifest(root string, opt LoadOptions) (Config, string, diagnostics.List) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Config{}, "", one("PRJ001", err.Error(), root)
	}
	cfg := Config{LanguageVersion: "v1alpha1", DefaultOutput: "dist"}
	cfgName := opt.ConfigFile
	if cfgName == "" {
		cfgName = "mosaic.toml"
	}
	cfgPath := cfgName
	if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(abs, cfgPath)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, cfgPath, nil
		}
		return Config{}, cfgPath, one("PRJ001", err.Error(), cfgName)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, cfgPath, one("PRJ002", err.Error(), cfgName)
	}
	return cfg, cfgPath, nil
}

// DependencyDigest binds dependency-affecting project configuration.
func DependencyDigest(cfg Config) string {
	model := struct {
		Name            string                              `json:"name"`
		LanguageVersion string                              `json:"languageVersion"`
		AllowExternal   bool                                `json:"allowExternal"`
		Dependencies    map[string]mosaicpackage.Dependency `json:"dependencies"`
		Replace         map[string]mosaicpackage.Dependency `json:"replace"`
	}{cfg.Name, cfg.LanguageVersion, cfg.AllowExternalLocalDependencies, cfg.Dependencies, cfg.Replace}
	b, _ := json.Marshal(model)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// MarshalConfig serializes project configuration deterministically.
func MarshalConfig(cfg Config) ([]byte, error) {
	b, err := toml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func New(root, name string, files []source.File) *Project {
	cp := append([]source.File(nil), files...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Name < cp[j].Name })
	return &Project{Root: root, Name: name, Files: cp}
}
func Load(ctx context.Context, root string, opt LoadOptions) (*Project, diagnostics.List) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, one("PRJ001", err.Error(), root)
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return nil, one("PRJ001", "project root is not a directory", root)
	}
	cfg := Config{LanguageVersion: "v1alpha1", DefaultOutput: "dist"}
	name := filepath.Base(abs)
	if loaded, _, d := LoadManifest(abs, opt); d.HasErrors() {
		return nil, d
	} else {
		cfg = loaded
		if cfg.Name != "" {
			name = cfg.Name
		}
	}
	var names []string
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rel, _ := filepath.Rel(abs, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "dist" || d.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".mosaic" {
			return nil
		}
		if len(cfg.Sources) > 0 && !matches(cfg.Sources, rel) {
			return nil
		}
		if matches(cfg.Exclude, rel) {
			return nil
		}
		names = append(names, rel)
		return nil
	})
	if err != nil {
		return nil, one("PRJ001", err.Error(), root)
	}
	sort.Strings(names)
	files := make([]source.File, 0, len(names))
	for _, n := range names {
		data, e := os.ReadFile(filepath.Join(abs, filepath.FromSlash(n)))
		if e != nil {
			return nil, one("PRJ001", e.Error(), n)
		}
		files = append(files, source.NewFile(n, data))
	}
	return &Project{Root: abs, Name: name, Files: files, Config: cfg}, nil
}
func matches(patterns []string, name string) bool {
	for _, p := range patterns {
		ok, _ := doublestar.Match(filepath.ToSlash(p), name)
		if ok {
			return true
		}
	}
	return false
}
func one(code, msg, name string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: msg, Span: diagnostics.Span{SourceName: name, Start: diagnostics.Position{Line: 1, Column: 1}, End: diagnostics.Position{Line: 1, Column: 1}}, Notes: []string{fmt.Sprintf("project: %s", name)}}}
}
