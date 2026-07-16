# Mosaic user guide

This guide explains how to author, distribute, consume, and operate Mosaic
packages. It is written for package authors, project maintainers, CI operators,
and Go applications embedding Mosaic.

Mosaic is a typed configuration compiler. Packages contain declarative Mosaic
source and metadata; they do not contain executable installation logic. Mosaic
builds deterministic bundles and does not deploy them.

## Suggested reading paths

New package authors:

1. [Getting started](01-getting-started.md)
2. [Package concepts](02-package-concepts.md)
3. [Authoring packages](03-authoring-packages.md)
4. [Manifest reference](04-package-manifest.md)
5. [Publishing to OCI](09-oci-distribution.md)

Project maintainers:

1. [Consuming packages](05-consuming-packages.md)
2. [Resolution and updates](06-resolution-and-updates.md)
3. [Lockfiles and reproducibility](07-lockfiles.md)
4. [Build modes](08-build-modes.md)
5. [Cache and restoration](10-cache-and-restore.md)
6. [Vendoring](11-vendoring.md)

CI and platform teams:

1. [CI workflows](12-ci-workflows.md)
2. [Security model](13-security.md)
3. [Diagnostics and troubleshooting](16-troubleshooting.md)

Library consumers:

1. [Public Go APIs](15-go-api.md)
2. [CLI reference](14-cli-reference.md)

The complete local and OCI walkthrough is in
[End-to-end tutorial](17-end-to-end-tutorial.md).

For the complete compiler workflow, including modules, variants, policies,
tests, inspection, provenance, and bundle comparison, see
[Core project workflow](18-core-project-workflow.md).

## Feature summary

Mosaic currently supports:

- local path and OCI package sources;
- semantic-version constraints and one selected version per package identity;
- deterministic TOML lockfiles;
- deterministic `.mosaicpkg` archives;
- SHA-256 manifest, content, archive, and OCI-manifest digests;
- verified content-addressed caching;
- locked, offline, and vendored builds;
- explicit exports and dependency-alias namespaces;
- public Go APIs for resolution, archives, caching, OCI, and compilation.

The following are intentionally not available: package scripts, plugins, Git or
HTTP archive dependencies, multi-version loading, runtime compiler registry
access, automatic signatures, and package catalogue services. Explicit
re-exports and schema exports are also not implemented yet. Package-defined
capabilities are documented in [the language reference](../docs/capabilities.md).

## Conventions

Examples use `prod` as an environment and assume commands run from the project
root. Use the global `--project` option from any directory:

```sh
mosaic --project ./catalog-platform validate prod
```

Use `--format json` when output will be consumed by automation. Examples using
an insecure local test registry add `--plain-http`; never use that option for a
production registry that supports TLS.
