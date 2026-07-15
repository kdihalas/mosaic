# Diagnostics and troubleshooting

## Read diagnostics

Text diagnostics contain severity, stable code, message, source location,
notes, related locations, and an optional suggestion. JSON output preserves the
same fields:

```sh
mosaic --format json validate prod --locked
```

Fix the earliest root-cause diagnostic first; later compiler errors may be
consequences of a missing package or private symbol.

## Package diagnostics

| Code | Meaning | Typical action |
| --- | --- | --- |
| `PKG001` | Invalid or unreadable package manifest. | Fix TOML and unknown fields. |
| `PKG002` | Invalid package identity. | Use lowercase slash-separated segments. |
| `PKG003` | Invalid or non-exact package version. | Use an exact semantic version. |
| `PKG010` | Source or local dependency escapes its root. | Move it inside the root or explicitly permit external local dependencies. |
| `PKG020` | Export does not match a declaration. | Fix its name or category. |
| `PKG021` | Public API exposes a private type. | Export the full type closure or change the API. |
| `PKG030` | Unsupported language version. | Use a compiler supporting the requested version. |
| `PKG031` | Symbol is private. | Export it or use a supported public entry point. |

## Dependency diagnostics

| Code | Meaning | Typical action |
| --- | --- | --- |
| `DEP001` | Invalid dependency alias. | Use a Mosaic identifier. |
| `DEP002` | Invalid version constraint. | Correct semantic-version syntax. |
| `DEP010` | Package/source/version not found. | Check source, version tags, and access. |
| `DEP012` | No version satisfies all constraints. | Review reported dependency paths and change constraints explicitly. |
| `DEP015` | Circular package dependency. | Break the package cycle. |
| `DEP020` | Identity or effective-source mismatch. | Verify the artifact and replacement configuration. |
| `DEP041` | Exact package unavailable offline. | Run `mosaic deps restore` while online or vendor it. |

## Lockfile diagnostics

| Code | Meaning | Typical action |
| --- | --- | --- |
| `LOCK001` | Lockfile missing. | Run `mosaic deps resolve`. |
| `LOCK002` | Lockfile stale. | Review manifest changes, then resolve. |
| `LOCK003` | Locked digest or local content mismatch. | Investigate content; resolve only if the change is intended. |
| `LOCK004` | Locked source mismatch. | Restore the declared source or explicitly resolve the source change. |
| `LOCK005` | Unsupported or malformed lock format. | Use a compatible Mosaic version and regenerate intentionally. |

## OCI diagnostics

| Code | Meaning | Typical action |
| --- | --- | --- |
| `OCI001` | Invalid OCI reference. | Use `oci://host/repository[:tag|@digest]`. |
| `OCI010` | Authentication failed. | Refresh registry login or credential provider. |
| `OCI011` | Access denied. | Check repository permissions. |
| `OCI012` | Artifact not found. | Check repository and tag/digest. |
| `OCI020` | OCI/config digest mismatch. | Treat as an integrity incident; do not update the lock blindly. |
| `OCI021` | Unsupported artifact media type. | Confirm the reference points to a Mosaic package. |
| `OCI030` | Version tag contains different content. | Publish a new semantic version. |

## Archive and cache diagnostics

| Code | Meaning | Typical action |
| --- | --- | --- |
| `ARCH001` | Unsafe path. | Remove absolute/traversal entries. |
| `ARCH002` | Link or special file rejected. | Package regular files only. |
| `ARCH003` | Duplicate or case-colliding path. | Rename one path. |
| `ARCH004` | Archive digest mismatch. | Obtain the expected immutable artifact. |
| `ARCH005` | Configured size/count limit exceeded. | Reduce package size or deliberately adjust API limits. |
| `CACHE001` | Cache entry corrupt. | Prune and restore from a trusted source. |
| `CACHE002` | Cache write failed. | Check cache path, permissions, and free space. |

## Common workflows

### Local package changed

```text
error[LOCK003]: local package content does not match lockfile
```

Review the package diff, validate it, then run:

```sh
mosaic deps resolve
mosaic build prod --locked
```

### Offline package missing

```sh
# While network access is available
mosaic deps restore

# Confirm readiness
mosaic cache verify
mosaic build prod --offline --locked
```

### Package symbol reported private

Check the dependency alias and the package `[exports]` category. Export the
declaration and every type in its public interface, increment the package
version, republish, and resolve the consumer.

### OCI login works in Docker but not Mosaic

Confirm the registry hostname matches the Docker credential entry exactly and
that the configured credential helper is available in the Mosaic process
environment. Do not copy credentials into `mosaic.toml`.

### Vendor build reports changed content

Do not edit the vendor directory. Recreate it from verified locked content:

```sh
mosaic deps restore
mosaic deps vendor
mosaic build prod --vendor --offline --locked
```

### Plain local registry fails

TLS is assumed by default. Add global `--plain-http` only for a deliberately
insecure local test registry:

```sh
mosaic --plain-http package pull oci://localhost:5000/acme/http:1.0.0
```
