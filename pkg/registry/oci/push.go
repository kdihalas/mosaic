package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/packagearchive"
	baseregistry "github.com/kdihalas/mosaic/pkg/registry"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/errdef"
)

func (c *Client) Publish(ctx context.Context, raw string, artifact *packagearchive.Artifact, options baseregistry.PublishOptions) (*baseregistry.Published, diagnostics.List) {
	if artifact == nil {
		return nil, ociDiagnostic("PKG001", "package artifact is nil", raw)
	}
	ref, err := ParseReference(raw)
	if err != nil {
		return nil, ociDiagnostic("OCI001", err.Error(), raw)
	}
	tag := ref.Reference
	if tag == "" {
		tag = artifact.Manifest.Version
	}
	if tag != artifact.Manifest.Version {
		return nil, ociDiagnostic("PKG003", "destination version tag does not match package version", raw)
	}
	repo, err := c.repository(ref)
	if err != nil {
		return nil, ociDiagnostic("OCI001", err.Error(), raw)
	}
	store := memory.New()
	configBytes, err := json.Marshal(configDocument{Package: artifact.Manifest.Name, Version: artifact.Manifest.Version, LanguageVersion: artifact.Manifest.LanguageVersion, Manifest: artifact.Manifest, ManifestDigest: artifact.ManifestDigest, ContentDigest: artifact.ContentDigest, ArchiveDigest: artifact.ArchiveDigest})
	if err != nil {
		return nil, ociDiagnostic("OCI020", err.Error(), raw)
	}
	configDesc, err := oras.PushBytes(ctx, store, MediaTypeConfig, configBytes)
	if err != nil {
		return nil, ociDiagnostic("OCI020", err.Error(), raw)
	}
	layerDesc, err := oras.PushBytes(ctx, store, MediaTypeLayer, artifact.Bytes)
	if err != nil {
		return nil, ociDiagnostic("OCI020", err.Error(), raw)
	}
	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, MediaTypeArtifact, oras.PackManifestOptions{Layers: []v1.Descriptor{layerDesc}, ConfigDescriptor: &configDesc, ManifestAnnotations: map[string]string{v1.AnnotationCreated: "1970-01-01T00:00:00Z"}})
	if err != nil {
		return nil, ociDiagnostic("OCI020", err.Error(), raw)
	}
	if existing, err := repo.Resolve(ctx, tag); err == nil && existing.Digest != manifestDesc.Digest {
		return nil, ociDiagnostic("OCI030", "version tag already exists with different content", raw)
	} else if err != nil && !errors.Is(err, errdef.ErrNotFound) {
		return nil, ociError(err, raw)
	}
	if err := store.Tag(ctx, manifestDesc, tag); err != nil {
		return nil, ociDiagnostic("OCI020", err.Error(), raw)
	}
	pushed, err := oras.Copy(ctx, store, tag, repo, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, ociError(err, raw)
	}
	if pushed.Digest != manifestDesc.Digest {
		return nil, ociDiagnostic("OCI020", "registry returned a different manifest digest", raw)
	}
	for _, alias := range options.Tags {
		if alias == tag {
			continue
		}
		if existing, err := repo.Resolve(ctx, alias); err == nil && existing.Digest != manifestDesc.Digest && !options.AllowTagMove {
			return nil, ociDiagnostic("OCI030", fmt.Sprintf("tag %s already exists with different content", alias), raw)
		}
		if _, err := oras.Copy(ctx, store, tag, repo, alias, oras.DefaultCopyOptions); err != nil {
			return nil, ociError(err, raw)
		}
	}
	digest := manifestDesc.Digest.String()
	return &baseregistry.Published{Reference: "oci://" + ref.RepositoryReference() + "@" + digest, VersionTag: tag, OCIManifestDigest: digest, ContentDigest: artifact.ContentDigest}, nil
}
