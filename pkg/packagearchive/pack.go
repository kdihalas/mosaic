package packagearchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
)

// Pack validates and creates deterministic archive bytes from a package directory.
func Pack(ctx context.Context, root string, options PackOptions) (*Artifact, diagnostics.List) {
	limits := options.Limits.WithDefaults()
	pkg, ds := mosaicpackage.Load(ctx, root)
	if ds.HasErrors() {
		return nil, ds
	}
	files, more := selectFiles(ctx, pkg.Root, pkg.Manifest, limits)
	ds = append(ds, more...)
	if ds.HasErrors() {
		return nil, ds.Sorted()
	}
	manifestDigest, err := mosaicpackage.ManifestDigest(pkg.Manifest)
	if err != nil {
		return nil, archiveDiagnostic("PKG001", err.Error(), mosaicpackage.ManifestName)
	}
	content := contentDigest(files)
	inventory := make([]FileDigest, 0, len(files))
	for _, file := range files {
		inventory = append(inventory, FileDigest{Path: file.Path, Size: int64(len(file.Data)), Digest: digestBytes(file.Data)})
	}
	doc, err := json.Marshal(DigestDocument{FormatVersion: "v1alpha1", ManifestDigest: manifestDigest, ContentDigest: content, Files: inventory})
	if err != nil {
		return nil, archiveDiagnostic("ARCH004", err.Error(), DigestFile)
	}
	archiveFiles := append(append([]File(nil), files...), File{Path: DigestFile, Data: doc})
	sortFiles(archiveFiles)
	b, err := writeArchive(archiveFiles)
	if err != nil {
		return nil, archiveDiagnostic("ARCH004", err.Error(), root)
	}
	if int64(len(b)) > limits.MaxArchiveBytes {
		return nil, archiveDiagnostic("ARCH005", "archive exceeds maximum compressed size", root)
	}
	return &Artifact{Manifest: pkg.Manifest, Bytes: b, Files: inventory, ManifestDigest: manifestDigest, ContentDigest: content, ArchiveDigest: digestBytes(b)}, ds.Sorted()
}

func selectFiles(ctx context.Context, root string, manifest mosaicpackage.Manifest, limits mosaicpackage.Limits) ([]File, diagnostics.List) {
	var files []File
	seenCase := map[string]string{}
	var total int64
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := cancelled(ctx); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("ARCH002: symlink not permitted: %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("ARCH002: forbidden file type: %s", rel)
		}
		if !includePath(manifest, rel) {
			return nil
		}
		canonical, ok := canonicalPath(rel)
		if !ok {
			return fmt.Errorf("ARCH001: unsafe archive path: %s", rel)
		}
		if previous, exists := seenCase[caseKey(canonical)]; exists && previous != canonical {
			return fmt.Errorf("ARCH003: case-colliding paths: %s and %s", previous, canonical)
		}
		seenCase[caseKey(canonical)] = canonical
		if info.Size() > limits.MaxFileBytes {
			return fmt.Errorf("ARCH005: file exceeds configured size: %s", rel)
		}
		total += info.Size()
		if total > limits.MaxUncompressedBytes {
			return fmt.Errorf("ARCH005: content exceeds configured size")
		}
		if len(files)+1 > limits.MaxFiles {
			return fmt.Errorf("ARCH005: file count exceeds configured limit")
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		files = append(files, File{Path: canonical, Data: data})
		return nil
	})
	if err != nil {
		code := "ARCH001"
		for _, candidate := range []string{"ARCH002", "ARCH003", "ARCH005"} {
			if strings.HasPrefix(err.Error(), candidate+":") {
				code = candidate
				break
			}
		}
		return nil, archiveDiagnostic(code, strings.TrimPrefix(err.Error(), code+": "), root)
	}
	sortFiles(files)
	return files, nil
}

func includePath(manifest mosaicpackage.Manifest, name string) bool {
	if name == mosaicpackage.ManifestName {
		return true
	}
	base := filepath.Base(name)
	conventional := strings.HasPrefix(base, "README") || strings.HasPrefix(base, "LICENSE") || strings.HasPrefix(name, "schemas/")
	deployableInput := len(manifest.Exports.Environments) > 0 && (name == "mosaic.lock" || strings.HasPrefix(name, "vendor/mosaic/"))
	matched := conventional || deployableInput
	for _, pattern := range manifest.Sources {
		if ok, _ := doublestar.Match(pattern, name); ok {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	for _, pattern := range manifest.Exclude {
		if ok, _ := doublestar.Match(pattern, name); ok {
			return false
		}
	}
	return !strings.HasPrefix(name, "dist/") && !strings.HasPrefix(name, ".git/")
}

func writeArchive(files []File) ([]byte, error) {
	var out bytes.Buffer
	gz, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	gz.ModTime = time.Unix(0, 0).UTC()
	gz.OS = 255
	tw := tar.NewWriter(gz)
	for _, file := range files {
		header := &tar.Header{Name: "package/" + file.Path, Mode: 0o644, Size: int64(len(file.Data)), ModTime: time.Unix(0, 0).UTC(), AccessTime: time.Time{}, ChangeTime: time.Time{}, Uid: 0, Gid: 0, Uname: "", Gname: "", Typeflag: tar.TypeReg, Format: tar.FormatPAX}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tw.Write(file.Data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
