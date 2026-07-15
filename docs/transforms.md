# Transforms and conflicts

Defaults and module construction establish the base graph. Applied variants and
transforms are peer owners: their textual or application order creates no
precedence. Equal writes and non-overlapping object/list identities coalesce;
different scalar/key/identity writes and delete/update pairs conflict.

`resolve <resource>.<field> = <value>` is the sole higher-precedence environment
operation. It resolves only its addressed ambiguity and is recorded in
provenance.

Use a quoted string index to resolve an object key that is not a Mosaic
identifier, such as a qualified Kubernetes label or annotation:

```mosaic
environment prod {
    use catalog
    apply platformDefaults
    apply production

    resolve catalog.workload.main.labels["app.example.com/tier"] = "production"
}
```

The index must be a string literal. Dynamic expressions and numeric indexes are
not valid resource mutation paths.
