# Authoring packages

## Start from the scaffold

```sh
mosaic package init packages/http-service
```

The scaffold provides working types, a module, a test, README, license, and
manifest. Rename the package identity and declarations to match your public
API.

## Organize package source

A practical layout is:

```text
mosaic.package.toml
src/
├── types.mosaic
├── modules.mosaic
├── variants.mosaic
└── policies.mosaic
tests/
└── package_tests.mosaic
schemas/
README.md
LICENSE
```

The layout has no semantic precedence. Source files are loaded
deterministically, and correctness must not depend on filename order.

## Design a small public surface

Keep implementation declarations private and export only stable entry points.
For example:

```toml
[exports]
modules = ["HttpService"]
types = ["HttpServiceInput", "ResourceProfile"]
variants = ["productionDefaults"]
policies = ["requiredResources"]
```

If `HttpService` accepts `HttpServiceInput`, export that input type. If the
input contains `ResourceProfile`, export that type too. The validator checks
this public type closure.

## Add package dependencies

Packages may declare OCI or local dependencies using the same dependency model:

```toml
[dependencies]
observability = { source = "oci://ghcr.io/acme/mosaic/observability", version = "^2.0.0" }
```

Local paths are intended for development:

```toml
[dependencies]
observability = { path = "../observability" }
```

Publishing rejects unresolved local-path dependencies. Publish the dependency
and change the manifest to an OCI source before publishing the dependent
package.

## Select package files carefully

Use `sources` for declarative source and tests, and `exclude` for content that
must never be packaged:

```toml
sources = ["src/**/*.mosaic", "tests/**/*.mosaic"]
exclude = ["dist/**", "testdata/**", ".git/**"]
```

The package archive also includes conventional metadata such as `README.md`
and `LICENSE` when selected by the packer. Mosaic rejects files outside the
package root, links, special files, unsafe paths, duplicate canonical paths,
and portable case collisions.

Do not put credentials, consuming-project lockfiles, generated bundles, or
temporary editor files into a package.

## Validate continuously

```sh
mosaic package validate packages/http-service
```

Validation parses the manifest and source, checks identity and version, checks
exports and public type closure, compiles the package, and runs package tests.
Use JSON diagnostics in tooling:

```sh
mosaic --format json package validate packages/http-service
```

## Pack deterministically

```sh
mosaic package pack packages/http-service \
  --output dist/http-service.mosaicpkg
```

Tests run before packing. `--skip-tests` is an explicit exception for local
work and should not be used in release automation.

Verify determinism before releasing:

```sh
mosaic package pack packages/http-service --output /tmp/http-a.mosaicpkg
mosaic package pack packages/http-service --output /tmp/http-b.mosaicpkg
cmp /tmp/http-a.mosaicpkg /tmp/http-b.mosaicpkg
```

## Verify the artifact

```sh
mosaic package inspect dist/http-service.mosaicpkg
mosaic package verify dist/http-service.mosaicpkg
```

Inspection reads metadata without executing package code. Verification checks
archive structure, identity, version, inventory, and digests.

## Release checklist

Before publishing:

- use an exact, intended semantic version;
- validate exports and public type closure;
- remove local replacements and local package dependencies;
- run package tests;
- pack twice and compare bytes for high-assurance releases;
- publish only to the intended OCI repository;
- record the immutable digest printed by Mosaic;
- never overwrite an existing semantic-version tag with different content.
