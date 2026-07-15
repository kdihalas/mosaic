# Lockfiles and reproducibility

## Purpose

`mosaic.lock` records the exact dependency graph selected by resolution. Commit
it with the project manifest. A build with dependencies must not select package
versions dynamically.

The lockfile binds:

- the project dependency configuration digest;
- declared and effective sources;
- package identities and exact versions;
- direct aliases;
- transitive dependency edges;
- replacement state;
- manifest, content, archive, and OCI manifest digests.

## Example

```toml
format_version = 'v1alpha1'
project = 'catalog-platform'
dependency_digest = 'sha256:...'

[[package]]
identity = 'acme/http-service'
version = '1.4.3'
source = 'oci://ghcr.io/acme/mosaic/http-service'
declared_source = 'oci://ghcr.io/acme/mosaic/http-service'
aliases = ['http']
manifest_digest = 'sha256:...'
content_digest = 'sha256:...'
archive_digest = 'sha256:...'
oci_manifest_digest = 'sha256:...'
dependencies = ['acme/observability@2.6.1']
```

The exact TOML quoting style is serializer-owned. Do not hand-format lockfiles.

## Determinism

Lockfiles contain no timestamps, credentials, machine-specific cache paths, or
absolute local paths. Packages are sorted by identity and version; aliases and
dependency edges are sorted.

Verify stable resolution in release automation when desired:

```sh
cp mosaic.lock /tmp/mosaic.lock.before
mosaic deps resolve
cmp /tmp/mosaic.lock.before mosaic.lock
```

## Stale lockfile conditions

Mosaic rejects a lockfile when:

- `mosaic.lock` is missing for a dependency-bearing project;
- dependency declarations changed;
- a constraint or declared source changed;
- replacement configuration changed;
- external-local-dependency policy changed;
- a local package's selected content changed;
- a fetched identity, version, or digest differs;
- the lockfile format is unsupported.

Resolve intended changes explicitly:

```sh
mosaic deps resolve
```

## Build flags

`--locked` guarantees the command will not update `mosaic.lock`:

```sh
mosaic build prod --locked
```

`--update-lock` resolves and writes the lock before continuing:

```sh
mosaic build prod --update-lock
```

`--locked` and `--update-lock` are incompatible and fail as invalid CLI usage.
Use `--locked` in CI and prefer explicit `mosaic deps resolve` during planned
dependency changes.

## Merge conflicts

Do not resolve a lockfile merge by keeping arbitrary package blocks. Instead:

1. merge `mosaic.toml` intentionally;
2. run `mosaic deps resolve`;
3. review `mosaic deps list` and `mosaic deps graph`;
4. run locked validation and tests;
5. commit the regenerated deterministic lockfile.

## Integrity failures

`LOCK003` means selected bytes no longer match the locked digest. For a local
dependency, review the source change and resolve again. For OCI or cache
content, do not update the lock merely to suppress the failure: first confirm
the registry reference and expected package provenance.
