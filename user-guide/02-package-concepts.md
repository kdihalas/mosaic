# Package concepts

## Package identity and location

A package identity is the `name` in `mosaic.package.toml`, for example:

```text
acme/http-service
```

An OCI source is a distribution location, for example:

```text
oci://ghcr.io/acme/mosaic/http-service
```

These values are deliberately separate. Mosaic reads the identity from the
package manifest and verifies that it matches the lockfile. It never assumes an
identity from an OCI repository name.

An identity consists of lowercase slash-separated segments. Segments may use
lowercase ASCII letters, digits, `-`, `_`, and `.`. Empty segments, `..`, URI
schemes, versions, registry hostnames, and filesystem separators other than `/`
are invalid.

## Versions and constraints

Package manifests contain exact semantic versions:

```toml
version = "1.4.3"
```

Consumers may declare constraints:

```text
1.4.3
^1.4.0
~1.4.0
>=1.4.0
>=1.4.0 <2.0.0
1.x
1.4.x
```

Build metadata does not affect precedence. Stable releases are preferred.
Prereleases are considered only when the constraint includes a prerelease or
the caller explicitly enables prereleases.

## Direct and transitive dependencies

A dependency declared by the root project is direct. A dependency declared by
a package is transitive. Mosaic resolves both, but their visibility differs:

- direct packages are visible to the root through the declared alias;
- a package sees dependencies it declares through its own aliases;
- transitive dependencies do not automatically become visible to the root;
- one exact version of an identity is selected for the entire graph.

If both the root and a package need `acme/observability`, all constraints must
be satisfied by the same version.

## Aliases and namespaces

The project alias is a Mosaic identifier:

```toml
[dependencies]
http = { source = "oci://ghcr.io/acme/mosaic/http-service", version = "^1.4.0" }
```

It becomes a source namespace:

```mosaic
use http.HttpService as catalog {
    name = "catalog-api"
}

type CatalogSettings {
    resources: http.ResourceProfile
}
```

Aliases are local conveniences. `http.HttpService` and `web.HttpService` can
refer to the same semantic symbol. The stable semantic identity remains:

```text
package.acme/http-service@1.4.3.module.HttpService
```

## Exports and privacy

Declarations are private unless listed in `[exports]`. Public module inputs and
public type fields must use types consumers can resolve. Validation fails if an
export references a missing declaration, the wrong declaration category, an
ambiguous name, or a private type in its public interface.

Tests are not visible to consuming projects unless explicitly exported. Tests
inside restored dependency packages are not run as root-project tests.

Explicit dependency re-exports are planned but not currently supported. A root
project must declare a package directly to use its symbols.

## Lockfile-first builds

Resolution chooses versions. Compilation does not. Once dependencies exist,
normal validation, test, and build operations use exact entries from
`mosaic.lock`. A changed dependency declaration or local package makes the lock
stale and requires an explicit resolution.

## Package content is data

Mosaic packages contain source and metadata only. They cannot execute scripts,
hooks, plugins, network requests, or filesystem operations. Registry access is
performed before compilation by dependency and OCI APIs; the compiler itself
never contacts registries.
