# Vendoring dependencies

Vendoring copies the exact locked dependency graph into the project so builds
can be isolated from both registries and unrelated user-cache content.

## Create the vendor tree

```sh
mosaic deps vendor
```

Default layout:

```text
vendor/mosaic/
├── vendor.lock
└── packages/
    └── acme/
        ├── http-service/
        │   └── 1.4.3/
        └── observability/
            └── 2.6.1/
```

Mosaic restores exact lockfile content as needed, writes a complete temporary
tree, verifies it, and atomically replaces the previous vendor directory.
Stale vendored packages are removed.

Choose a different output directory when creating a distributable workspace:

```sh
mosaic deps vendor --output ./third_party/mosaic
```

The build-time `--vendor` mode currently expects the conventional
`vendor/mosaic` location.

## Vendor without network access

After the cache is complete:

```sh
mosaic deps vendor --offline
```

This fails if an exact locked package is absent from verified local content.

## Build from vendor

```sh
mosaic build prod --vendor --offline --locked
```

In vendored mode Mosaic:

- does not contact registries;
- does not use unrelated cache entries;
- requires every locked package in the vendor tree;
- rehashes content against `mosaic.lock`;
- rejects modified, missing, or substituted packages.

Use the same flags for validation and tests:

```sh
mosaic validate prod --vendor --offline --locked
mosaic test --vendor --offline --locked
```

## Commit or generate?

Commit `vendor/mosaic` when repository-contained offline builds are a project
requirement. Otherwise generate it as a CI artifact from a committed lockfile.
In either model, treat `vendor.lock` and the project `mosaic.lock` as a pair.

## Updating vendored dependencies

```sh
mosaic deps update --dry-run
mosaic deps update
mosaic deps restore
mosaic deps vendor
mosaic build prod --vendor --offline --locked
```

Review both lockfile changes and dependency graph output. Never manually edit
vendored package source to patch a dependency; such edits intentionally fail
digest verification. Publish or locally replace a new package version instead.

## Determinism check

For release assurance:

```sh
mosaic deps vendor --output /tmp/vendor-a
mosaic deps vendor --output /tmp/vendor-b
diff -r /tmp/vendor-a /tmp/vendor-b
```
