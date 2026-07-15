package packagearchive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
)

// Unpack verifies data and atomically creates destination. Destination must not exist.
func Unpack(ctx context.Context, data []byte, destination string, options UnpackOptions) (*Artifact, diagnostics.List) {
	artifact, ds := Verify(ctx, data, options.VerifyOptions)
	if ds.HasErrors() {
		return nil, ds
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, archiveDiagnostic("CACHE002", err.Error(), destination)
	}
	temp, err := os.MkdirTemp(parent, ".mosaic-unpack-")
	if err != nil {
		return nil, archiveDiagnostic("CACHE002", err.Error(), destination)
	}
	defer func() { _ = os.RemoveAll(temp) }()
	files, _, readDS := readFiles(ctx, data, options.Limits.WithDefaults())
	if readDS.HasErrors() {
		return nil, readDS
	}
	for _, file := range files {
		path := filepath.Join(temp, filepath.FromSlash(file.Path))
		rel, err := filepath.Rel(temp, path)
		if err != nil || rel == ".." || filepath.IsAbs(rel) {
			return nil, archiveDiagnostic("ARCH001", "unsafe extraction path", file.Path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, archiveDiagnostic("CACHE002", err.Error(), file.Path)
		}
		if err := os.WriteFile(path, file.Data, 0o644); err != nil {
			return nil, archiveDiagnostic("CACHE002", err.Error(), file.Path)
		}
	}
	if _, err := os.Stat(destination); err == nil {
		return nil, archiveDiagnostic("CACHE002", fmt.Sprintf("destination already exists: %s", destination), destination)
	}
	if err := os.Rename(temp, destination); err != nil {
		return nil, archiveDiagnostic("CACHE002", err.Error(), destination)
	}
	return artifact, ds
}
