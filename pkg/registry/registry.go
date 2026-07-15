// Package registry defines registry-neutral package distribution contracts.
package registry

import (
	"context"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/packagearchive"
)

type Credentials struct {
	Username     string
	Password     string
	RefreshToken string
	AccessToken  string
}
type CredentialProvider interface {
	Credentials(context.Context, string) (Credentials, error)
}
type CredentialProviderFunc func(context.Context, string) (Credentials, error)

func (f CredentialProviderFunc) Credentials(ctx context.Context, registry string) (Credentials, error) {
	return f(ctx, registry)
}

// Set maps source schemes to reusable registry clients.
type Set map[string]Client

type PublishOptions struct {
	Tags         []string
	AllowTagMove bool
}
type Published struct {
	Reference         string `json:"reference"`
	VersionTag        string `json:"versionTag"`
	OCIManifestDigest string `json:"ociManifestDigest"`
	ContentDigest     string `json:"contentDigest"`
}

type Client interface {
	Pull(context.Context, string) (*packagearchive.Artifact, diagnostics.List)
	Publish(context.Context, string, *packagearchive.Artifact, PublishOptions) (*Published, diagnostics.List)
	Tags(context.Context, string) ([]string, diagnostics.List)
}
