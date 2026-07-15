# Consuming packages

## Project dependency configuration

Dependencies live in the root `mosaic.toml`:

```toml
name = "catalog-platform"
language_version = "v1alpha1"
sources = ["applications/**/*.mosaic", "environments/**/*.mosaic"]
exclude = []
default_output = "dist"

[dependencies]
http = {
  source = "oci://ghcr.io/acme/mosaic/http-service",
  version = "^1.4.0",
}
```

The alias `http` becomes the namespace used in Mosaic source.

## Add an OCI dependency

```sh
mosaic deps add \
  http \
  oci://ghcr.io/acme/mosaic/http-service \
  '^1.4.0'
```

`deps add` validates the alias, resolves the graph, and updates `mosaic.toml`
and `mosaic.lock` transactionally. If resolution fails, the manifest is not
left partially modified.

## Add a local dependency

```sh
mosaic deps add http ./packages/http-service
```

Local paths are relative to the project root and must stay inside it by
default. To use a sibling development package:

```toml
allow_external_local_dependencies = true

[dependencies]
http = { path = "../http-service" }
```

This option increases trust in local filesystem input. Avoid enabling it in
projects that do not require sibling packages.

## Use package symbols

Module:

```mosaic
use http.HttpService as catalog {
    name = "catalog-api"
    image = "ghcr.io/acme/catalog-api:1.4.0"
}
```

Type:

```mosaic
type CatalogSettings {
    resources: http.ResourceProfile
}
```

Variant:

```mosaic
environment prod {
    use catalog
    apply http.productionDefaults
}
```

Policy:

```mosaic
environment prod {
    use catalog

    policies {
        use http.requiredResources
    }
}
```

Access to a declaration not listed in package exports produces a package
privacy diagnostic. A transitive package alias is not available in the root
project unless the root declares that package directly.

## Remove a dependency

```sh
mosaic deps remove http
```

Removal re-resolves the remaining graph and updates the project and lockfile
transactionally. Remove source references to the alias before validating the
project.

## Development replacements

Keep the production OCI declaration and override an identity locally:

```toml
[dependencies]
http = {
  source = "oci://ghcr.io/acme/mosaic/http-service",
  version = "^1.4.0",
}

[replace]
"acme/http-service" = { path = "./packages/http-service" }
```

The lockfile records the declared source, effective local source, digest, and
replacement marker. Replacements are project-only and are never propagated as
published transitive dependency declarations.

CI can reject them explicitly:

```sh
mosaic validate --no-replacements --locked
```

Publishing also rejects unresolved local development dependencies.

## Inspect the graph

```sh
mosaic deps list
mosaic deps graph
mosaic deps graph --format json
mosaic deps graph --format dot > dependencies.dot
```

`list` distinguishes direct entries, which have root aliases, from transitive
entries. DOT and JSON output are deterministic and suitable for automation.
