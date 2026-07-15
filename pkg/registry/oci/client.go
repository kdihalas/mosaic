package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/kdihalas/mosaic/pkg/dependency"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/lockfile"
	mosaicpackage "github.com/kdihalas/mosaic/pkg/package"
	baseregistry "github.com/kdihalas/mosaic/pkg/registry"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type Options struct {
	Credentials baseregistry.CredentialProvider
	PlainHTTP   bool
	HTTPClient  *http.Client
	Limits      mosaicpackage.Limits
}
type Client struct{ options Options }

func New(options Options) *Client {
	options.Limits = options.Limits.WithDefaults()
	return &Client{options: options}
}
func (c *Client) repository(ref Reference) (*remote.Repository, error) {
	repo, err := remote.NewRepository(ref.RepositoryReference())
	if err != nil {
		return nil, err
	}
	repo.PlainHTTP = c.options.PlainHTTP
	repo.MaxMetadataBytes = c.options.Limits.MaxRegistryResponseBytes
	httpClient := c.options.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	repo.Client = &auth.Client{Client: httpClient, Cache: auth.NewCache(), Credential: func(ctx context.Context, registry string) (auth.Credential, error) {
		if c.options.Credentials == nil {
			return auth.EmptyCredential, nil
		}
		credential, err := c.options.Credentials.Credentials(ctx, registry)
		if err != nil {
			return auth.EmptyCredential, err
		}
		return auth.Credential{Username: credential.Username, Password: credential.Password, RefreshToken: credential.RefreshToken, AccessToken: credential.AccessToken}, nil
	}}
	return repo, nil
}

type configDocument struct {
	Package         string                 `json:"package"`
	Version         string                 `json:"version"`
	LanguageVersion string                 `json:"languageVersion"`
	Manifest        mosaicpackage.Manifest `json:"manifest"`
	ManifestDigest  string                 `json:"manifestDigest"`
	ContentDigest   string                 `json:"contentDigest"`
	ArchiveDigest   string                 `json:"archiveDigest"`
}

func (c *Client) fetchMetadata(ctx context.Context, raw string) (dependency.VersionMetadata, v1.Descriptor, diagnostics.List) {
	ref, err := ParseReference(raw)
	if err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI001", err.Error(), raw)
	}
	if ref.Reference == "" {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI001", "exact OCI reference requires a tag or digest", raw)
	}
	repo, err := c.repository(ref)
	if err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI001", err.Error(), raw)
	}
	descriptor, err := repo.Resolve(ctx, ref.Reference)
	if err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociError(err, raw)
	}
	reader, err := repo.Fetch(ctx, descriptor)
	if err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociError(err, raw)
	}
	manifestBytes, err := readBounded(reader, c.options.Limits.MaxRegistryResponseBytes)
	_ = reader.Close()
	if err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI020", err.Error(), raw)
	}
	var manifest v1.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI021", "invalid OCI manifest", raw)
	}
	if manifest.ArtifactType != MediaTypeArtifact || manifest.Config.MediaType != MediaTypeConfig || len(manifest.Layers) != 1 || manifest.Layers[0].MediaType != MediaTypeLayer {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI021", "unsupported Mosaic OCI artifact media types", raw)
	}
	configReader, err := repo.Fetch(ctx, manifest.Config)
	if err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociError(err, raw)
	}
	configBytes, err := readBounded(configReader, c.options.Limits.MaxRegistryResponseBytes)
	_ = configReader.Close()
	if err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI020", err.Error(), raw)
	}
	var config configDocument
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI021", "invalid Mosaic OCI config", raw)
	}
	md, _ := mosaicpackage.ManifestDigest(config.Manifest)
	if config.Package != config.Manifest.Name || config.Version != config.Manifest.Version || config.ManifestDigest != md {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("OCI020", "OCI config package metadata mismatch", raw)
	}
	version, err := mosaicpackage.ParseVersion(config.Version)
	if err != nil {
		return dependency.VersionMetadata{}, v1.Descriptor{}, ociDiagnostic("PKG003", err.Error(), raw)
	}
	source := "oci://" + ref.RepositoryReference()
	return dependency.VersionMetadata{Version: version, Manifest: config.Manifest, Source: source, ManifestDigest: config.ManifestDigest, ContentDigest: config.ContentDigest, ArchiveDigest: config.ArchiveDigest, OCIManifestDigest: descriptor.Digest.String()}, manifest.Layers[0], nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("registry response exceeds configured limit")
	}
	return b, nil
}
func ociDiagnostic(code, message, source string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: message, Span: diagnostics.Span{SourceName: source}}}
}
func ociError(err error, source string) diagnostics.List {
	message := strings.ToLower(err.Error())
	code := "OCI012"
	if strings.Contains(message, "unauthorized") {
		code = "OCI010"
	} else if strings.Contains(message, "denied") || strings.Contains(message, "forbidden") {
		code = "OCI011"
	}
	return ociDiagnostic(code, err.Error(), source)
}

func (c *Client) Supports(ref dependency.SourceReference) bool {
	return strings.HasPrefix(ref.Source, "oci://")
}
func (c *Client) SupportsLockedSource(source string) bool { return strings.HasPrefix(source, "oci://") }
func (c *Client) ListVersions(ctx context.Context, ref dependency.SourceReference) ([]dependency.VersionMetadata, diagnostics.List) {
	tags, ds := c.Tags(ctx, ref.Source)
	if ds.HasErrors() {
		return nil, ds
	}
	var out []dependency.VersionMetadata
	for _, tag := range tags {
		if _, err := mosaicpackage.ParseVersion(tag); err != nil {
			continue
		}
		metadata, _, more := c.fetchMetadata(ctx, ref.Source+":"+tag)
		if more.HasErrors() {
			return nil, more
		}
		out = append(out, metadata)
		if len(out) > c.options.Limits.MaxAvailableVersions {
			return nil, ociDiagnostic("ARCH005", "available version limit exceeded", ref.Source)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version.Semver().GreaterThan(out[j].Version.Semver()) })
	return out, nil
}
func (c *Client) FetchManifest(ctx context.Context, ref dependency.ExactPackageReference) (*mosaicpackage.Manifest, diagnostics.List) {
	raw := ref.Source + ":" + ref.Version.String()
	if ref.Digest != "" {
		raw = ref.Source + "@" + ref.Digest
	}
	metadata, _, ds := c.fetchMetadata(ctx, raw)
	if ds.HasErrors() {
		return nil, ds
	}
	if metadata.Manifest.Name != ref.Identity.String() {
		return nil, ociDiagnostic("DEP020", "package identity mismatch", raw)
	}
	manifest := metadata.Manifest
	return &manifest, nil
}
func (c *Client) FetchArchive(ctx context.Context, locked lockfile.Package) ([]byte, diagnostics.List) {
	raw := locked.Source + ":" + locked.Version
	if locked.OCIManifestDigest != "" {
		raw = locked.Source + "@" + locked.OCIManifestDigest
	}
	artifact, ds := c.Pull(ctx, raw)
	if ds.HasErrors() {
		return nil, ds
	}
	if artifact.ContentDigest != locked.ContentDigest || artifact.Manifest.Name != locked.Identity {
		return nil, ociDiagnostic("LOCK003", "pulled package does not match lockfile", raw)
	}
	return artifact.Bytes, nil
}
