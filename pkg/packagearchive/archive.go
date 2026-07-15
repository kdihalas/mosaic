// Package packagearchive creates and verifies deterministic Mosaic package archives.
package packagearchive

import (
	"context"
	"io"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
)

const DigestFile = "package.digest"

// File records a canonical package file.
type File struct {
	Path string `json:"path"`
	Data []byte `json:"-"`
}

// FileDigest binds an archive inventory entry to its bytes.
type FileDigest struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

// DigestDocument is stored inside every archive.
type DigestDocument struct {
	FormatVersion  string       `json:"formatVersion"`
	ManifestDigest string       `json:"manifestDigest"`
	ContentDigest  string       `json:"contentDigest"`
	Files          []FileDigest `json:"files"`
}

// Artifact is a packed, verified package archive.
type Artifact struct {
	Manifest          mosaicpackage.Manifest `json:"manifest"`
	Bytes             []byte                 `json:"-"`
	Files             []FileDigest           `json:"files"`
	ManifestDigest    string                 `json:"manifestDigest"`
	ContentDigest     string                 `json:"contentDigest"`
	ArchiveDigest     string                 `json:"archiveDigest"`
	OCIManifestDigest string                 `json:"ociManifestDigest,omitempty"`
}

type PackOptions struct {
	Limits mosaicpackage.Limits
}

type VerifyOptions struct {
	Limits                mosaicpackage.Limits
	ExpectedArchiveDigest string
	ExpectedContentDigest string
	ExpectedIdentity      mosaicpackage.Identity
	ExpectedVersion       mosaicpackage.Version
}

type UnpackOptions struct {
	VerifyOptions
}

// ReadArchive is implemented by verification inputs without requiring a filesystem.
type ReadArchive interface {
	Open() (io.ReadCloser, error)
}

func archiveDiagnostic(code, message, source string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: message, Span: diagnostics.Span{SourceName: source}}}
}

func cancelled(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
