# Language

Mosaic files contain type and enum declarations, modules, top-level module
instances, variants, transforms, environments, policies, and tests. Modules
produce named resources and typed references. Environments explicitly choose
instances, variants/transforms, policies, and render-target options.

Transforms support `set`, `replace`, `delete`, `append`, `merge`, `add`, and
`enable`; conflicts require an environment `resolve`. Policies select graph
resources and use `require`, `deny`, or `warn`. Tests build an environment and
evaluate assertions.

Object keys are strings. Identifier keys are shorthand; keys containing `/`,
`.` or other punctuation use exact quoted syntax:

```mosaic
labels {
    "app.kubernetes.io/name" = input.name
}
```

## Kubernetes custom resources

Use an explicit `resource` declaration when an installed Kubernetes operator
owns the schema:

```mosaic
resource "serviceMonitor" {
    apiVersion = "monitoring.coreos.com/v1"
    kind = "ServiceMonitor"
    name = input.monitorName
    labels {
        "app.kubernetes.io/name" = input.name
    }
    spec {
        selector {
            matchLabels {
                "app.kubernetes.io/name" = input.name
            }
        }
        endpoints = [{ port = "metrics", interval = "30s" }]
    }
}
```

The local quoted name creates the stable ID
`application.<alias>.resource.<local-name>`; keep it identifier-compatible so
variants can address it. `apiVersion`, `kind`, and `name` are required. Mosaic
preserves nested scalar types, provenance, variants, policies, diffs, and
deterministic rendering. It does not install the CRD or replace validation by
the operator's current OpenAPI schema.
