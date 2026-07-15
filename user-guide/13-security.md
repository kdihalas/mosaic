# Security model

## Trust boundary

Treat package manifests, archives, registry responses, and restored cache or
vendor content as untrusted input. Mosaic validates these inputs before they
reach semantic compilation.

Mosaic packages cannot execute:

- shell commands or lifecycle scripts;
- Go, JavaScript, WASM, or arbitrary plugins;
- package-supplied network requests;
- filesystem operations outside declarative source loading;
- environment lookups, clocks, or random generators.

The compiler receives already resolved source packages and has no registry
client.

## Digest model

Four digests protect different boundaries:

| Digest | Protects |
| --- | --- |
| Manifest digest | Canonical package metadata. |
| Content digest | Canonical file paths and file bytes. |
| Archive digest | Exact compressed `.mosaicpkg` bytes. |
| OCI manifest digest | Immutable registry artifact manifest. |

The lockfile binds these digests to the declared source, package identity, and
exact version. Verification must succeed at every restoration boundary.

## Archive protections

The archive implementation:

- rejects absolute, traversal, Windows-absolute, and non-canonical paths;
- rejects symlinks, hard links, devices, sockets, and named pipes;
- rejects duplicate and portable case-colliding paths;
- normalizes ownership, modes, times, separators, and gzip metadata;
- checks compressed size, uncompressed size, file count, and per-file size;
- extracts into a controlled temporary destination before atomic replacement;
- checks the digest inventory and rejects unexpected files.

## Default limits

Public APIs use practical defaults:

| Limit | Default |
| --- | ---: |
| Archive bytes | 512 MiB |
| Uncompressed bytes | 2 GiB |
| Files | 100,000 |
| Single file | 128 MiB |
| Dependency depth | 50 |
| Dependencies | 500 |
| Available versions | 10,000 |
| Registry response | 32 MiB |

Go callers may provide smaller `package.Limits` for their environment. Zero
fields are filled with defaults.

## Local path containment

Local paths are relative to the declaring project or package root. Mosaic
resolves filesystem paths and symlinks and rejects escape from the root unless
`allow_external_local_dependencies = true` is explicit.

Enable external paths only when the entire containing filesystem location is
trusted. The option is included in the project dependency digest, so changing
it makes the lockfile stale.

## Dependency confusion protection

Mosaic does not derive identity from an OCI repository name. The package
manifest is authoritative but must match the selected identity and lockfile.
Two sources cannot claim the same identity in one graph unless an explicit root
replacement chooses the effective source.

## Credentials

The CLI adapts standard Docker registry credentials. Public APIs accept an
explicit credential provider. Secrets are not stored in manifests, lockfiles,
archives, bundles, cache metadata, or diagnostics.

Avoid credentials in command-line arguments and registry source URLs. Prefer
credential helpers, short-lived tokens, and least-privilege repository access.

## Offline and vendor isolation

`--offline` disables registry and credential access. `--vendor` further limits
package content to the verified project vendor tree and avoids unrelated cache
entries. Use both with `--locked` for isolated builds.

## Current trust limitations

Mosaic verifies integrity, not publisher identity. Automatic signature
verification, Sigstore, transparency logs, and entitlement systems are not
implemented. Apply registry access controls and independently protect trusted
lockfiles until signature policy APIs are added.
