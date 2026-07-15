# Getting started

## Install Mosaic

Install the CLI with Go:

```sh
go install github.com/kdihalas/mosaic/cmd/mosaic@latest
mosaic version
```

To build the checked-out source instead:

```sh
go build -o ./bin/mosaic ./cmd/mosaic
./bin/mosaic version
```

## Create a package

The package scaffold is valid, compilable, tested, and exports an example
module:

```sh
mosaic package init packages/http-service
mosaic package validate packages/http-service
mosaic package pack packages/http-service
```

The generated layout is:

```text
packages/http-service/
├── mosaic.package.toml
├── src/
│   ├── module.mosaic
│   └── types.mosaic
├── tests/
│   └── package_tests.mosaic
├── README.md
└── LICENSE
```

Change the generated package identity before publishing. Package identity is
independent of its OCI repository:

```toml
name = "acme/http-service"
version = "1.0.0"
```

## Consume the package locally

From a Mosaic project root:

```sh
mosaic deps add http ../packages/http-service
mosaic deps list
mosaic deps graph
```

The command updates `mosaic.toml` and `mosaic.lock` transactionally. If the path
is outside the project root, explicitly permit external local dependencies in
`mosaic.toml`:

```toml
allow_external_local_dependencies = true
```

Use an exported module through its alias namespace:

```mosaic
use http.ExampleService as catalog {
    name = "catalog-api"
    image = "ghcr.io/acme/catalog-api:1.0.0"
}
```

Then build with the exact locked dependency:

```sh
mosaic validate prod --locked
mosaic test --locked
mosaic build prod --locked
```

## Prepare for offline work

For OCI dependencies, restore the exact locked archives once while online:

```sh
mosaic deps restore
mosaic build prod --offline --locked
```

Or vendor the graph into the project:

```sh
mosaic deps vendor
mosaic build prod --vendor --offline --locked
```

## Next steps

- Learn the identity, export, and visibility rules in
  [Package concepts](02-package-concepts.md).
- Follow [Authoring packages](03-authoring-packages.md) before publishing.
- See [End-to-end tutorial](17-end-to-end-tutorial.md) for local development,
  OCI publication, restoration, and offline builds.
