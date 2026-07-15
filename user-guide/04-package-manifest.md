# Package manifest reference

Every package root contains `mosaic.package.toml`. Parsing is strict: unknown
fields and malformed TOML are errors.

## Complete example

```toml
name = "acme/http-service"
version = "1.4.0"
description = "Reusable HTTP microservice module"
language_version = "v1alpha1"
license = "Apache-2.0"
homepage = "https://example.com/mosaic/http-service"
repository = "https://example.com/acme/http-service"
authors = ["Acme Platform Team"]

sources = [
  "src/**/*.mosaic",
  "tests/**/*.mosaic",
]

exclude = [
  "dist/**",
  "testdata/**",
  ".git/**",
]

[exports]
modules = ["HttpService"]
types = [
  "HttpServiceInput",
  "ResourceProfile",
  "ContainerResources",
  "HealthCheck",
]
enums = []
variants = ["developmentDefaults", "productionDefaults"]
transforms = []
policies = ["requiredResources", "requiredOwnership"]
tests = []

[dependencies]
observability = {
  source = "oci://ghcr.io/acme/mosaic/observability",
  version = "^2.0.0",
}
```

## Top-level fields

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | Yes | Registry-independent package identity. |
| `version` | Yes | Exact semantic version. |
| `language_version` | Yes | Required Mosaic language version. Currently `v1alpha1`. |
| `sources` | Yes | Package-relative doublestar source patterns. |
| `description` | No | Human-readable summary. |
| `license` | No | License identifier or description. |
| `homepage` | No | Package homepage metadata. |
| `repository` | No | Source repository metadata. |
| `authors` | No | Author strings. |
| `exclude` | No | Package-relative patterns removed from selection. |
| `exports` | Yes for useful libraries | Explicit public symbols by category. |
| `dependencies` | No | Alias-to-source declarations. |

Manifest metadata is canonicalized before its SHA-256 digest is calculated.
List ordering therefore does not introduce nondeterministic manifest digests.

## Identity rules

Valid examples:

```text
acme/http-service
community/postgres
platform/team.shared_library
```

Invalid examples include uppercase segments, `../http`, `/acme/http`,
`acme/http/`, `ghcr.io/acme/http`, `acme/http@1.0.0`, and values containing a
URI scheme.

## Version rules

The manifest version must be exact:

```toml
version = "1.4.0"
```

Constraints such as `^1.4.0` belong only in dependency declarations.

## Source patterns

Patterns use `/` separators and are evaluated relative to the package root.
At least one source pattern is required. A matched path must remain inside the
root after filesystem and symlink evaluation.

## Export categories

Supported declaration categories are:

- `modules`
- `types`
- `enums`
- `variants`
- `transforms`
- `policies`
- `tests`

`capabilities` and `schemas` are represented in the manifest model but
non-empty exports in those categories are currently rejected. Explicit
re-export tables are also not supported yet.

Each exported name must exist in exactly the listed category. A name exported
under multiple categories or a public API referencing private types fails
validation.

## Dependency entries

OCI dependency:

```toml
[dependencies]
observability = {
  source = "oci://ghcr.io/acme/mosaic/observability",
  version = ">=2.3.0 <3.0.0",
}
```

Local dependency:

```toml
[dependencies]
observability = { path = "../observability" }
```

An entry uses either `path` or `source` plus `version`, never both. Aliases must
be valid Mosaic identifiers.
