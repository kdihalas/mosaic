// Package project loads deterministic collections of Mosaic source files.
package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Name            string   `toml:"name"`
	LanguageVersion string   `toml:"language_version"`
	Sources         []string `toml:"sources"`
	Exclude         []string `toml:"exclude"`
	DefaultOutput   string   `toml:"default_output"`
}
type Project struct {
	Root   string
	Name   string
	Files  []source.File
	Config Config
}
type LoadOptions struct{ ConfigFile string }

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
	cfgName := opt.ConfigFile
	if cfgName == "" {
		cfgName = "mosaic.toml"
	}
	cfgPath := cfgName
	if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(abs, cfgPath)
	}
	if data, e := os.ReadFile(cfgPath); e == nil {
		if e = toml.Unmarshal(data, &cfg); e != nil {
			return nil, one("PRJ002", e.Error(), cfgName)
		}
		if cfg.Name != "" {
			name = cfg.Name
		}
	} else if !os.IsNotExist(e) {
		return nil, one("PRJ001", e.Error(), cfgName)
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
