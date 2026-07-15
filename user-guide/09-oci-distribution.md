# OCI distribution

Mosaic stores package archives as artifacts in standard OCI registries. It
does not require or provide a Mosaic-specific registry server.

## References

Repository:

```text
oci://ghcr.io/acme/mosaic/http-service
```

Version tag:

```text
oci://ghcr.io/acme/mosaic/http-service:1.4.0
```

Immutable digest:

```text
oci://ghcr.io/acme/mosaic/http-service@sha256:...
```

Dependency declarations separate repository and constraint. Lockfiles record
the exact version and immutable OCI manifest digest.

## Media types

Mosaic uses:

```text
application/vnd.mosaic.package.manifest.v1+json
application/vnd.mosaic.package.config.v1+json
application/vnd.mosaic.package.layer.v1.tar+gzip
```

The config records package identity, version, language version, manifest
digest, and content digest. The layer is the deterministic `.mosaicpkg`
archive.

Library package layers contain only their selected package sources and normal
metadata. Packages that export an environment are deployable packages; their
layers additionally contain `mosaic.lock` and `vendor/mosaic/` so downstream
artifact producers can build them offline without hidden registry access.

## Publish

Publish using an exact reference:

```sh
mosaic package publish packages/http-service \
  --reference oci://ghcr.io/acme/mosaic/http-service:1.4.0
```

Or provide a repository and let the manifest version determine the version
tag:

```sh
mosaic package publish packages/http-service \
  --registry oci://ghcr.io/acme/mosaic/http-service
```

Publishing validates and tests the package, creates the deterministic archive,
pushes config and layer content, publishes the OCI manifest, verifies its
digest, and prints an immutable reference.

The requested version tag must match the manifest version. Existing semantic
version tags cannot move to different content.

Additional aliases are explicit:

```sh
mosaic package publish packages/http-service \
  --registry oci://ghcr.io/acme/mosaic/http-service \
  --tag 1.4 \
  --tag latest
```

Mutable tag movement requires `--allow-tag-move`; semantic-version tag
immutability remains the safe default. Avoid mutable tags in project lockfiles.

## Pull and verify

```sh
mosaic package pull \
  oci://ghcr.io/acme/mosaic/http-service:1.4.0
```

Pulling resolves the OCI manifest, validates media types and digests, verifies
the Mosaic archive, stores it in the content-addressed cache, and prints the
immutable digest.

Copy the archive to a specific location:

```sh
mosaic package pull \
  oci://ghcr.io/acme/mosaic/http-service:1.4.0 \
  --output ./dist/http-service.mosaicpkg
```

Populate only the cache:

```sh
mosaic package pull REF --cache-only
```

Inspect or verify a remote artifact without executing package code:

```sh
mosaic package inspect REF
mosaic package verify REF
```

## Authentication

The CLI uses standard Docker registry configuration and credential helpers.
Authenticate with the tooling appropriate for your registry, for example
`docker login`, without placing credentials in Mosaic files.

Credentials must never appear in:

- `mosaic.toml`;
- `mosaic.lock`;
- package manifests or archives;
- bundles or diagnostics.

Go applications can supply an explicit `registry.CredentialProvider`; they do
not need to depend on process-global environment state.

## Local test registries

Plain HTTP is disabled by default. For an intentionally insecure local test
registry only:

```sh
mosaic --plain-http package publish packages/http-service \
  --reference oci://localhost:5000/acme/http-service:1.4.0
```

Do not use `--plain-http` for production registries.

## Supply-chain checks

On every pull Mosaic verifies the OCI manifest digest, package archive digest,
manifest digest, content digest, identity, version, media types, and safe
archive structure. A repository containing a package with a different manifest
identity fails rather than being inferred from the repository name.
