# Build modes

Dependency-aware flags apply to `build`, `validate`, and `test`.

## Normal lockfile-first mode

```sh
mosaic build prod
```

When dependencies exist, Mosaic reads and validates `mosaic.lock`, verifies
local content, restores missing OCI packages when network access is available,
loads package sources, and compiles the root environment.

Normal mode never silently performs version resolution. Use `deps resolve` or
`--update-lock` for that explicit operation.

## Locked mode

```sh
mosaic validate prod --locked
mosaic test --locked
mosaic build prod --locked
```

Locked mode does not modify the lockfile and fails if it is missing or stale.
This is the recommended CI mode.

## Update-lock mode

```sh
mosaic build prod --update-lock
```

This resolves dependencies, writes the new lockfile, restores content, and
continues the requested operation. It is convenient during interactive
development but makes dependency change and compilation one operation. For
reviewable changes, prefer separate `deps resolve` and locked build commands.

## Offline cache mode

```sh
mosaic build prod --offline --locked
```

Offline mode permits local dependencies and verified cache entries. It does
not query registries, resolve DNS, refresh tags, retrieve remote credentials,
or perform a hidden network fallback.

Prepare OCI packages beforehand:

```sh
mosaic deps restore
```

If content is unavailable, Mosaic reports `DEP041` with the identity, exact
version, expected digest, and restoration guidance.

## Vendored mode

```sh
mosaic build prod --vendor --offline --locked
```

Vendored mode reads only `vendor/mosaic` for remote package content. It does
not use unrelated cache entries or registries. Every vendored package is
verified against `mosaic.lock`.

## Cache location

Override the user cache globally for any command:

```sh
mosaic --cache-dir ./build/package-cache deps restore
mosaic --cache-dir ./build/package-cache build prod --offline --locked
```

Use the same cache path for restoration and offline compilation.

## Local development mode

Local path dependencies are always rehashed. If they change after resolution,
the next locked build fails. Refresh the lock only after reviewing the change:

```sh
mosaic deps resolve
mosaic build prod --locked
```

## Flag compatibility

Do not combine `--locked` with `--update-lock`. `--vendor` implies no registry
access and is normally combined with both `--offline` and `--locked` to make
the intent obvious and enforce reproducibility.
