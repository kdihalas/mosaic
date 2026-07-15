package oci

import (
	"context"
	"os"
	"path/filepath"

	"github.com/kdihalas/mosaic/pkg/dependency"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	"github.com/kdihalas/mosaic/pkg/packagearchive"
)

func (c *Client) Pull(ctx context.Context, raw string) (*packagearchive.Artifact, diagnostics.List) {
	metadata, layer, ds := c.fetchMetadata(ctx, raw)
	if ds.HasErrors() {
		return nil, ds
	}
	ref, _ := ParseReference(raw)
	repo, err := c.repository(ref)
	if err != nil {
		return nil, ociDiagnostic("OCI001", err.Error(), raw)
	}
	reader, err := repo.Fetch(ctx, layer)
	if err != nil {
		return nil, ociError(err, raw)
	}
	data, err := readBounded(reader, c.options.Limits.MaxArchiveBytes)
	_ = reader.Close()
	if err != nil {
		return nil, ociDiagnostic("ARCH005", err.Error(), raw)
	}
	identity, _ := mosaicpackage.ParseIdentity(metadata.Manifest.Name)
	artifact, ds := packagearchive.Verify(ctx, data, packagearchive.VerifyOptions{Limits: c.options.Limits, ExpectedArchiveDigest: metadata.ArchiveDigest, ExpectedContentDigest: metadata.ContentDigest, ExpectedIdentity: identity, ExpectedVersion: metadata.Version})
	if ds.HasErrors() {
		return nil, ds
	}
	artifact.OCIManifestDigest = metadata.OCIManifestDigest
	return artifact, nil
}

func (c *Client) FetchPackage(ctx context.Context, ref dependency.ExactPackageReference) (*mosaicpackage.Package, diagnostics.List) {
	raw := ref.Source + ":" + ref.Version.String()
	if ref.Digest != "" {
		raw = ref.Source + "@" + ref.Digest
	}
	artifact, ds := c.Pull(ctx, raw)
	if ds.HasErrors() {
		return nil, ds
	}
	temp, err := os.MkdirTemp("", "mosaic-oci-")
	if err != nil {
		return nil, ociDiagnostic("CACHE002", err.Error(), raw)
	}
	defer func() { _ = os.RemoveAll(temp) }()
	root := filepath.Join(temp, "package")
	if _, ds := packagearchive.Unpack(ctx, artifact.Bytes, root, packagearchive.UnpackOptions{VerifyOptions: packagearchive.VerifyOptions{Limits: c.options.Limits, ExpectedContentDigest: artifact.ContentDigest}}); ds.HasErrors() {
		return nil, ds
	}
	return mosaicpackage.Load(ctx, root)
}
