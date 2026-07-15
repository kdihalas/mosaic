# Dependencies

Projects declare local or OCI dependencies in `mosaic.toml`:

```toml
[dependencies]
http = { path = "packages/http-service" }
policy = { source = "oci://ghcr.io/acme/mosaic/policy-baseline", version = "^3.1.0" }
```

Aliases become namespaces. Constraints support exact, caret, tilde, comparator
ranges, and `x` wildcards. Resolution selects one version per identity, prefers
stable releases, and reports incompatible dependency paths.

Use `mosaic deps add`, `remove`, `resolve`, `restore`, `update`, `list`,
`graph`, and `vendor`. Local paths remain inside the project unless
`allow_external_local_dependencies = true` is explicit.
