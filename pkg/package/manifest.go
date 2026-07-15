package mosaicpackage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unicode"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
	"github.com/pelletier/go-toml/v2"
)

const ManifestName = "mosaic.package.toml"

// Dependency describes either a local path or OCI package dependency.
type Dependency struct {
	Source  string `toml:"source,omitempty" json:"source,omitempty"`
	Version string `toml:"version,omitempty" json:"version,omitempty"`
	Path    string `toml:"path,omitempty" json:"path,omitempty"`
}

// Manifest is the validated, declarative package manifest model.
type Manifest struct {
	Name            string                `toml:"name" json:"name"`
	Version         string                `toml:"version" json:"version"`
	Description     string                `toml:"description,omitempty" json:"description,omitempty"`
	LanguageVersion string                `toml:"language_version" json:"languageVersion"`
	License         string                `toml:"license,omitempty" json:"license,omitempty"`
	Homepage        string                `toml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository      string                `toml:"repository,omitempty" json:"repository,omitempty"`
	Authors         []string              `toml:"authors,omitempty" json:"authors,omitempty"`
	Sources         []string              `toml:"sources" json:"sources"`
	Exclude         []string              `toml:"exclude,omitempty" json:"exclude,omitempty"`
	Exports         ExportSet             `toml:"exports" json:"exports"`
	Dependencies    map[string]Dependency `toml:"dependencies,omitempty" json:"dependencies,omitempty"`
}

// Package is a loaded package ready for validation or compilation.
type Package struct {
	Root     string        `json:"root,omitempty"`
	Manifest Manifest      `json:"manifest"`
	Files    []source.File `json:"-"`
}

// LoadManifest reads a package manifest without loading source files.
func LoadManifest(ctx context.Context, root string) (Manifest, diagnostics.List) {
	select {
	case <-ctx.Done():
		return Manifest{}, manifestDiagnostic("PKG001", ctx.Err().Error(), root)
	default:
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, manifestDiagnostic("PKG001", err.Error(), root)
	}
	b, err := os.ReadFile(filepath.Join(abs, ManifestName))
	if err != nil {
		return Manifest{}, manifestDiagnostic("PKG001", err.Error(), ManifestName)
	}
	return ParseManifest(b, ManifestName)
}

// ParseManifest decodes strict TOML package manifest bytes.
func ParseManifest(b []byte, sourceName string) (Manifest, diagnostics.List) {
	var m Manifest
	dec := toml.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, manifestDiagnostic("PKG001", err.Error(), sourceName)
	}
	return m, ValidateManifest(m)
}

// CanonicalManifest returns stable JSON used by digest and OCI metadata.
func CanonicalManifest(m Manifest) ([]byte, error) {
	n := m
	n.Authors = sortedCopy(n.Authors)
	n.Sources = sortedCopy(n.Sources)
	n.Exclude = sortedCopy(n.Exclude)
	n.Exports = n.Exports.Sorted()
	return json.Marshal(n)
}

// ManifestDigest returns the SHA-256 digest of canonical manifest data.
func ManifestDigest(m Manifest) (string, error) {
	b, err := CanonicalManifest(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func manifestDiagnostic(code, message, source string) diagnostics.List {
	return diagnostics.List{{
		Code: code, Severity: diagnostics.SeverityError, Message: message,
		Span: diagnostics.Span{SourceName: source, Start: diagnostics.Position{Line: 1, Column: 1}, End: diagnostics.Position{Line: 1, Column: 1}},
	}}
}

func validateAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("dependency alias is empty")
	}
	for i, r := range alias {
		if i == 0 && r != '_' && !unicode.IsLetter(r) {
			return fmt.Errorf("dependency alias %q is not a Mosaic identifier", alias)
		}
		if i > 0 && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && !unicode.Is(unicode.Pc, r) {
			return fmt.Errorf("dependency alias %q is not a Mosaic identifier", alias)
		}
	}
	return nil
}
