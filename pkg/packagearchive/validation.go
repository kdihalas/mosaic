package packagearchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

// Verify validates archive structure, package metadata, inventory, and digests.
func Verify(ctx context.Context, data []byte, options VerifyOptions) (*Artifact, diagnostics.List) {
	limits := options.Limits.WithDefaults()
	if int64(len(data)) > limits.MaxArchiveBytes {
		return nil, archiveDiagnostic("ARCH005", "archive exceeds configured size", "archive")
	}
	archiveDigest := digestBytes(data)
	if options.ExpectedArchiveDigest != "" && archiveDigest != options.ExpectedArchiveDigest {
		return nil, archiveDiagnostic("ARCH004", "archive digest mismatch", "archive")
	}
	files, doc, ds := readFiles(ctx, data, limits)
	if ds.HasErrors() {
		return nil, ds
	}
	var manifestData []byte
	contentFiles := make([]File, 0, len(files)-1)
	for _, file := range files {
		if file.Path == mosaicpackage.ManifestName {
			manifestData = file.Data
		}
		if file.Path != DigestFile {
			contentFiles = append(contentFiles, file)
		}
	}
	if manifestData == nil {
		return nil, archiveDiagnostic("PKG001", "archive is missing mosaic.package.toml", "archive")
	}
	manifest, mds := mosaicpackage.ParseManifest(manifestData, mosaicpackage.ManifestName)
	ds = append(ds, mds...)
	if ds.HasErrors() {
		return nil, ds.Sorted()
	}
	manifestDigest, _ := mosaicpackage.ManifestDigest(manifest)
	content := contentDigest(contentFiles)
	if doc.ManifestDigest != manifestDigest || doc.ContentDigest != content {
		return nil, archiveDiagnostic("ARCH004", "embedded package digest mismatch", DigestFile)
	}
	if options.ExpectedContentDigest != "" && options.ExpectedContentDigest != content {
		return nil, archiveDiagnostic("ARCH004", "content digest mismatch", "archive")
	}
	identity, _ := mosaicpackage.ParseIdentity(manifest.Name)
	version, _ := mosaicpackage.ParseVersion(manifest.Version)
	if options.ExpectedIdentity != "" && identity != options.ExpectedIdentity {
		return nil, archiveDiagnostic("DEP020", fmt.Sprintf("package identity mismatch: expected %s, found %s", options.ExpectedIdentity, identity), mosaicpackage.ManifestName)
	}
	if options.ExpectedVersion != "" && version != options.ExpectedVersion {
		return nil, archiveDiagnostic("DEP020", fmt.Sprintf("package version mismatch: expected %s, found %s", options.ExpectedVersion, version), mosaicpackage.ManifestName)
	}
	expectedInventory := make([]FileDigest, 0, len(contentFiles))
	pkg := &mosaicpackage.Package{Manifest: manifest}
	for _, file := range contentFiles {
		if file.Path != mosaicpackage.ManifestName && !includePath(manifest, file.Path) {
			return nil, archiveDiagnostic("ARCH003", "archive contains a file not selected by the manifest", file.Path)
		}
		expectedInventory = append(expectedInventory, FileDigest{Path: file.Path, Size: int64(len(file.Data)), Digest: digestBytes(file.Data)})
		if strings.HasSuffix(file.Path, ".mosaic") {
			pkg.Files = append(pkg.Files, source.NewFile(file.Path, file.Data))
		}
	}
	if !equalInventory(doc.Files, expectedInventory) {
		return nil, archiveDiagnostic("ARCH004", "archive inventory mismatch", DigestFile)
	}
	ds = append(ds, mosaicpackage.ValidateExports(pkg)...)
	if ds.HasErrors() {
		return nil, ds.Sorted()
	}
	return &Artifact{Manifest: manifest, Bytes: append([]byte(nil), data...), Files: expectedInventory, ManifestDigest: manifestDigest, ContentDigest: content, ArchiveDigest: archiveDigest}, ds.Sorted()
}

func readFiles(ctx context.Context, data []byte, limits mosaicpackage.Limits) ([]File, DigestDocument, diagnostics.List) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, DigestDocument{}, archiveDiagnostic("ARCH004", err.Error(), "archive")
	}
	gz.Multistream(false)
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	var files []File
	seen, seenCase := map[string]bool{}, map[string]string{}
	var total int64
	var doc DigestDocument
	for {
		if err := cancelled(ctx); err != nil {
			return nil, doc, archiveDiagnostic("ARCH004", err.Error(), "archive")
		}
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, doc, archiveDiagnostic("ARCH004", err.Error(), "archive")
		}
		if header.Typeflag != tar.TypeReg {
			return nil, doc, archiveDiagnostic("ARCH002", "only regular files are permitted", header.Name)
		}
		if !strings.HasPrefix(header.Name, "package/") {
			return nil, doc, archiveDiagnostic("ARCH001", "entry is outside package root", header.Name)
		}
		name, ok := canonicalPath(strings.TrimPrefix(header.Name, "package/"))
		if !ok {
			return nil, doc, archiveDiagnostic("ARCH001", "unsafe archive path", header.Name)
		}
		if seen[name] {
			return nil, doc, archiveDiagnostic("ARCH003", "duplicate archive entry", name)
		}
		if previous, exists := seenCase[caseKey(name)]; exists && previous != name {
			return nil, doc, archiveDiagnostic("ARCH003", "case-colliding archive entries", name)
		}
		seen[name], seenCase[caseKey(name)] = true, name
		if header.Size < 0 || header.Size > limits.MaxFileBytes {
			return nil, doc, archiveDiagnostic("ARCH005", "archive file exceeds configured size", name)
		}
		total += header.Size
		if total > limits.MaxUncompressedBytes || len(files)+1 > limits.MaxFiles {
			return nil, doc, archiveDiagnostic("ARCH005", "archive exceeds configured limits", name)
		}
		b, err := io.ReadAll(io.LimitReader(tr, header.Size+1))
		if err != nil || int64(len(b)) != header.Size {
			return nil, doc, archiveDiagnostic("ARCH004", "truncated archive entry", name)
		}
		files = append(files, File{Path: name, Data: b})
	}
	sortFiles(files)
	found := false
	for _, file := range files {
		if file.Path == DigestFile {
			if err := json.Unmarshal(file.Data, &doc); err != nil {
				return nil, doc, archiveDiagnostic("ARCH004", "invalid package.digest", DigestFile)
			}
			found = true
		}
	}
	if !found {
		return nil, doc, archiveDiagnostic("ARCH004", "archive is missing package.digest", "archive")
	}
	return files, doc, nil
}

func equalInventory(a, b []FileDigest) bool {
	if len(a) != len(b) {
		return false
	}
	aa, bb := append([]FileDigest(nil), a...), append([]FileDigest(nil), b...)
	sort.Slice(aa, func(i, j int) bool { return aa[i].Path < aa[j].Path })
	sort.Slice(bb, func(i, j int) bool { return bb[i].Path < bb[j].Path })
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
