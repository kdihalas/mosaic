# Cache and restoration

## Cache purpose

The package cache stores verified archives and unpacked source by content
digest. It is an optimization and offline content store, not a source of
unspecified dependency selection. `mosaic.lock` remains authoritative.

Default locations follow `os.UserCacheDir`, resulting in conventional platform
locations such as:

```text
Linux:   $XDG_CACHE_HOME/mosaic/packages
macOS:   ~/Library/Caches/mosaic/packages
Windows: %LOCALAPPDATA%\mosaic\packages
```

Override the location with the global flag:

```sh
mosaic --cache-dir ./cache deps restore
```

## Content-addressed layout

```text
packages/
└── sha256/
    └── ab/
        └── abcdef.../
            ├── package.mosaicpkg
            ├── unpacked/
            └── metadata.json
```

The directory key is the content digest, not the package name. Mosaic still
verifies the bytes inside the directory before use.

## Restore exact dependencies

```sh
mosaic deps restore
```

Restoration never selects a new version. For each lock entry it:

1. verifies an existing cache entry if present;
2. obtains local package content or pulls the immutable OCI artifact if needed;
3. validates identity, version, archive, manifest, and content digests;
4. safely unpacks into a temporary cache entry;
5. atomically makes the entry available.

Restore without network fallback:

```sh
mosaic deps restore --offline
```

## Inspect cache state

```sh
mosaic cache list
mosaic --format json cache list
```

Entries show identity, version, content digest, archive size, and verification
status.

## Verify the cache

```sh
mosaic cache verify
```

Verification recalculates package integrity. Corrupt content is rejected; the
presence of a digest-looking directory name is never treated as proof.

## Prune unused entries

Preview removal:

```sh
mosaic cache prune --dry-run
```

Protect packages used by one or more lockfiles:

```sh
mosaic cache prune \
  --project-lockfiles ./mosaic.lock,../another-project/mosaic.lock
```

Without protected lockfiles, prune considers every cache entry removable.
Review `--dry-run` output first. The accepted `--older-than` option is reserved
for age-based pruning but is not currently enforced.

## Recover from corruption

If `cache verify` reports corruption:

1. identify whether the lockfile and registry reference are trusted;
2. prune or remove the corrupt cache entry through the cache workflow;
3. run `mosaic deps restore` while online;
4. run `mosaic cache verify` again;
5. build with `--offline --locked` to confirm the restored graph is complete.

Never regenerate the lockfile merely because cached bytes fail integrity.
