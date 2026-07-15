// Package packagecache manages verified content-addressed Mosaic package archives.
package packagecache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/packagearchive"
)

type Options struct {
	Root   string
	Limits mosaicpackage.Limits
}
type Cache struct {
	root   string
	limits mosaicpackage.Limits
}

type Metadata struct {
	Identity       string `json:"identity"`
	Version        string `json:"version"`
	ManifestDigest string `json:"manifestDigest"`
	ContentDigest  string `json:"contentDigest"`
	ArchiveDigest  string `json:"archiveDigest"`
	Size           int64  `json:"size"`
}

type Entry struct {
	Path     string   `json:"path"`
	Metadata Metadata `json:"metadata"`
	Valid    bool     `json:"valid"`
}

func New(options Options) (*Cache, error) {
	root := options.Root
	if root == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(base, "mosaic", "packages")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Cache{root: abs, limits: options.Limits.WithDefaults()}, nil
}
func (c *Cache) Root() string { return c.root }

func (c *Cache) entryPath(contentDigest string) (string, error) {
	hex, ok := strings.CutPrefix(contentDigest, "sha256:")
	if !ok || len(hex) != 64 {
		return "", fmt.Errorf("invalid content digest %q", contentDigest)
	}
	return filepath.Join(c.root, "sha256", hex[:2], hex), nil
}

func (c *Cache) Put(ctx context.Context, data []byte, expected lockfile.Package) (*packagearchive.Artifact, diagnostics.List) {
	identity, err := mosaicpackage.ParseIdentity(expected.Identity)
	if err != nil {
		return nil, cacheDiagnostic("CACHE002", err.Error(), c.root)
	}
	version, err := mosaicpackage.ParseVersion(expected.Version)
	if err != nil {
		return nil, cacheDiagnostic("CACHE002", err.Error(), c.root)
	}
	artifact, ds := packagearchive.Verify(ctx, data, packagearchive.VerifyOptions{Limits: c.limits, ExpectedArchiveDigest: expected.ArchiveDigest, ExpectedContentDigest: expected.ContentDigest, ExpectedIdentity: identity, ExpectedVersion: version})
	if ds.HasErrors() {
		return nil, ds
	}
	if expected.ManifestDigest != "" && artifact.ManifestDigest != expected.ManifestDigest {
		return nil, cacheDiagnostic("LOCK003", "package manifest digest does not match lockfile", expected.Source)
	}
	entry, err := c.entryPath(artifact.ContentDigest)
	if err != nil {
		return nil, cacheDiagnostic("CACHE002", err.Error(), c.root)
	}
	if existing, hit := c.Get(ctx, expected); hit {
		return existing, nil
	}
	parent := filepath.Dir(entry)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, cacheDiagnostic("CACHE002", err.Error(), parent)
	}
	temp, err := os.MkdirTemp(parent, ".mosaic-cache-")
	if err != nil {
		return nil, cacheDiagnostic("CACHE002", err.Error(), parent)
	}
	defer func() { _ = os.RemoveAll(temp) }()
	if err := os.WriteFile(filepath.Join(temp, "package.mosaicpkg"), data, 0o644); err != nil {
		return nil, cacheDiagnostic("CACHE002", err.Error(), temp)
	}
	if _, uds := packagearchive.Unpack(ctx, data, filepath.Join(temp, "unpacked"), packagearchive.UnpackOptions{VerifyOptions: packagearchive.VerifyOptions{Limits: c.limits, ExpectedArchiveDigest: artifact.ArchiveDigest, ExpectedContentDigest: artifact.ContentDigest, ExpectedIdentity: identity, ExpectedVersion: version}}); uds.HasErrors() {
		return nil, uds
	}
	metadata := Metadata{Identity: expected.Identity, Version: expected.Version, ManifestDigest: artifact.ManifestDigest, ContentDigest: artifact.ContentDigest, ArchiveDigest: artifact.ArchiveDigest, Size: int64(len(data))}
	mb, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(temp, "metadata.json"), mb, 0o644); err != nil {
		return nil, cacheDiagnostic("CACHE002", err.Error(), temp)
	}
	if err := os.Rename(temp, entry); err != nil {
		if _, statErr := os.Stat(entry); statErr == nil {
			if existing, ok := c.Get(ctx, expected); ok {
				return existing, nil
			}
		}
		return nil, cacheDiagnostic("CACHE002", err.Error(), entry)
	}
	return artifact, nil
}

func (c *Cache) Get(ctx context.Context, expected lockfile.Package) (*packagearchive.Artifact, bool) {
	entry, err := c.entryPath(expected.ContentDigest)
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(entry, "package.mosaicpkg"))
	if err != nil {
		return nil, false
	}
	identity, err := mosaicpackage.ParseIdentity(expected.Identity)
	if err != nil {
		return nil, false
	}
	version, err := mosaicpackage.ParseVersion(expected.Version)
	if err != nil {
		return nil, false
	}
	artifact, ds := packagearchive.Verify(ctx, data, packagearchive.VerifyOptions{Limits: c.limits, ExpectedArchiveDigest: expected.ArchiveDigest, ExpectedContentDigest: expected.ContentDigest, ExpectedIdentity: identity, ExpectedVersion: version})
	return artifact, !ds.HasErrors() && (expected.ManifestDigest == "" || artifact.ManifestDigest == expected.ManifestDigest)
}

// LoadPackage returns verified cached source suitable for compiler input.
func (c *Cache) LoadPackage(ctx context.Context, expected lockfile.Package) (*mosaicpackage.Package, diagnostics.List) {
	if _, ok := c.Get(ctx, expected); !ok {
		return nil, cacheDiagnostic("CACHE001", "cache entry is missing or corrupt", expected.Source)
	}
	entry, err := c.entryPath(expected.ContentDigest)
	if err != nil {
		return nil, cacheDiagnostic("CACHE001", err.Error(), expected.Source)
	}
	pkg, ds := mosaicpackage.Load(ctx, filepath.Join(entry, "unpacked"))
	if ds.HasErrors() {
		return nil, ds
	}
	if pkg.Manifest.Name != expected.Identity || pkg.Manifest.Version != expected.Version {
		return nil, cacheDiagnostic("CACHE001", "cached package identity or version mismatch", expected.Source)
	}
	return pkg, ds
}

func (c *Cache) List(ctx context.Context) []Entry {
	var out []Entry
	_ = filepath.WalkDir(c.root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "metadata.json" {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return nil
		}
		var m Metadata
		if json.Unmarshal(b, &m) != nil {
			return nil
		}
		locked := lockfile.Package{Identity: m.Identity, Version: m.Version, ManifestDigest: m.ManifestDigest, ContentDigest: m.ContentDigest, ArchiveDigest: m.ArchiveDigest}
		_, valid := c.Get(ctx, locked)
		out = append(out, Entry{Path: filepath.Dir(path), Metadata: m, Valid: valid})
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Metadata.Identity != out[j].Metadata.Identity {
			return out[i].Metadata.Identity < out[j].Metadata.Identity
		}
		return out[i].Metadata.Version < out[j].Metadata.Version
	})
	return out
}

func cacheDiagnostic(code, message, source string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: message, Span: diagnostics.Span{SourceName: source}}}
}
