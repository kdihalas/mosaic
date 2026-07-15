# CI workflows

CI should consume committed manifests and lockfiles without selecting new
versions. Dependency updates belong in an explicit review workflow.

## Standard cached CI

```sh
mosaic deps restore
mosaic validate --locked --no-replacements
mosaic test --locked
mosaic build prod --locked
```

Cache the Mosaic package cache between jobs using a key that includes
`mosaic.lock`. Always retain integrity verification; a cache hit is not proof
that package bytes are valid.

## Strict offline verification

Use two phases to prove the build no longer needs a registry:

```sh
# Network-enabled preparation phase
mosaic deps restore

# Network-disabled verification phase
mosaic build prod --offline --locked
```

Enforce network isolation outside Mosaic as defense in depth. Mosaic offline
mode itself does not initialize remote clients or credential providers.

## Vendored CI

For repositories that commit vendor content:

```sh
mosaic validate --vendor --offline --locked --no-replacements
mosaic test --vendor --offline --locked
mosaic build prod --vendor --offline --locked
```

This verifies that the committed vendor tree matches `mosaic.lock` and is
sufficient by itself.

## Package release pipeline

```sh
mosaic package validate ./packages/http-service --locked
mosaic package pack ./packages/http-service \
  --output /tmp/http-service.mosaicpkg
mosaic package verify /tmp/http-service.mosaicpkg
mosaic package publish ./packages/http-service \
  --reference oci://REGISTRY/acme/http-service:1.4.0
```

Run packing twice and compare archives in high-assurance pipelines. Publish
only immutable semantic-version tags by default. Store the resulting immutable
OCI digest in release metadata.

## Dependency update pipeline

Dependency updates should create a reviewable change rather than run inside
every build:

```sh
mosaic deps update --dry-run
mosaic deps update
mosaic deps restore
mosaic deps graph --format json
mosaic validate --locked --no-replacements
mosaic test --locked
mosaic build prod --locked
```

Review changes to both `mosaic.toml` and `mosaic.lock`. Major constraint changes
must be edited explicitly because automatic `--major` widening is not
implemented.

## Machine-readable output

Set the global output format before the command group:

```sh
mosaic --format json deps resolve
mosaic --format json deps list
mosaic --format json package validate ./packages/http-service
mosaic --format json cache list
```

Diagnostics contain a stable code, severity, message, source span, optional
notes, related locations, and suggestion. Treat an unknown diagnostic code as
an error rather than parsing human-formatted text.

## Recommended repository checks

- Reject missing or changed lockfiles after a locked build.
- Reject `[replace]` entries in release branches.
- Verify deterministic package archives.
- Verify cache or vendor integrity.
- Run package tests before publication.
- Test an offline build after restoration.
- Never inject registry credentials into Mosaic manifests or command output.
- Do not publish mutable aliases unless release policy explicitly requires
  them.
