# Packages

Mosaic packages are declarative source archives described by
`mosaic.package.toml`. They contain types, modules, variants, transforms,
policies, enums, and tests, and never execute hooks or arbitrary code.

```sh
mosaic package init packages/http-service
mosaic package validate packages/http-service
mosaic package pack packages/http-service
```

Consumers address exports through dependency aliases, for example
`use http.HttpService as catalog`. Private and transitive declarations are not
added to the root scope. Capability/schema declarations, re-exports,
signatures, Git dependencies, and multi-version loading are deferred.
