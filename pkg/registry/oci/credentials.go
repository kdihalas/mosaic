package oci

import (
	"context"

	baseregistry "github.com/kdihalas/mosaic/pkg/registry"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// DockerCredentials adapts standard Docker config and credential helpers.
type DockerCredentials struct{ store credentials.Store }

func NewDockerCredentials() (*DockerCredentials, error) {
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, err
	}
	return &DockerCredentials{store: store}, nil
}
func (d *DockerCredentials) Credentials(ctx context.Context, registry string) (baseregistry.Credentials, error) {
	credential, err := d.store.Get(ctx, registry)
	if err != nil {
		return baseregistry.Credentials{}, err
	}
	return baseregistry.Credentials{Username: credential.Username, Password: credential.Password, RefreshToken: credential.RefreshToken, AccessToken: credential.AccessToken}, nil
}
