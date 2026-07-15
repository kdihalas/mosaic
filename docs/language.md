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
