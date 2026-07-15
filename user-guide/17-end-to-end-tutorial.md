# End-to-end package tutorial

This tutorial creates two packages and a consuming project, develops with local
paths, vendors the graph, and then switches to a local OCI test registry.

## 1. Create packages

```sh
mkdir -p package-demo/packages
cd package-demo
mosaic package init packages/observability
mosaic package init packages/http-service
```

Set the identities and versions:

```toml
# packages/observability/mosaic.package.toml
name = "acme/observability"
version = "1.0.0"
```

```toml
# packages/http-service/mosaic.package.toml
name = "acme/http-service"
version = "1.0.0"
```

Rename the scaffold declarations as desired and update `[exports]`. To model a
transitive local dependency during development, add to the HTTP package:

```toml
[dependencies]
observability = { path = "../observability" }
```

Validate both:

```sh
mosaic package validate packages/observability
mosaic package validate packages/http-service
```

## 2. Prove deterministic packing

```sh
mosaic package pack packages/http-service --output /tmp/http-a.mosaicpkg
mosaic package pack packages/http-service --output /tmp/http-b.mosaicpkg
cmp /tmp/http-a.mosaicpkg /tmp/http-b.mosaicpkg
mosaic package verify /tmp/http-a.mosaicpkg
```

## 3. Create a consumer

```sh
mosaic init catalog-platform
```

Because the package is a sibling outside the project root, add:

```toml
# catalog-platform/mosaic.toml
allow_external_local_dependencies = true
```

Add the direct package:

```sh
cd catalog-platform
mosaic deps add http ../packages/http-service
mosaic deps list
mosaic deps graph
```

The graph should include direct `acme/http-service@1.0.0` and transitive
`acme/observability@1.0.0`.

Use the exported HTTP module through `http`. Adapt its inputs to the names you
chose in the package:

```mosaic
use http.HttpService as catalog {
    name = "catalog-api"
    image = "ghcr.io/acme/catalog-api:1.0.0"
    owner = "catalog-team"
}

environment prod {
    use catalog
    apply http.productionDefaults

    policies {
        use http.requiredResources
    }

    target kubernetes {
        namespace = "catalog-prod"
    }
}
```

Validate and build:

```sh
mosaic fmt
mosaic validate prod --locked
mosaic test --locked
mosaic build prod --locked
```

## 4. Vendor the local graph

```sh
mosaic deps vendor
mosaic build prod --vendor --offline --locked
```

Modify a vendored file and repeat the build to observe digest rejection, then
recreate the vendor tree rather than retaining the edit.

## 5. Publish the transitive package

Start a standard OCI Distribution registry using the method appropriate for
your development environment. The examples below assume `localhost:5000` and
plain HTTP.

From `package-demo`:

```sh
mosaic --plain-http package publish packages/observability \
  --reference oci://localhost:5000/acme/observability:1.0.0
```

Record the immutable digest printed by Mosaic.

## 6. Convert and publish the HTTP package

Replace its local dependency:

```toml
[dependencies]
observability = {
  source = "oci://localhost:5000/acme/observability",
  version = "^1.0.0",
}
```

Then publish:

```sh
mosaic --plain-http package validate packages/http-service
mosaic --plain-http package publish packages/http-service \
  --reference oci://localhost:5000/acme/http-service:1.0.0
```

Local-path transitive dependencies must be converted before publication.

## 7. Switch the consumer to OCI

Change the consumer declaration:

```toml
[dependencies]
http = {
  source = "oci://localhost:5000/acme/http-service",
  version = "^1.0.0",
}
```

Remove `allow_external_local_dependencies` if no local dependencies remain.
Resolve and restore:

```sh
cd catalog-platform
mosaic --plain-http deps resolve
mosaic --plain-http deps restore
mosaic deps list
mosaic deps graph
```

## 8. Prove the offline build

Stop or block access to the test registry, then run:

```sh
mosaic build prod --offline --locked
```

The build uses exact verified cache content. Re-vendor the OCI-backed graph and
test vendor isolation:

```sh
mosaic deps vendor --offline
mosaic build prod --vendor --offline --locked
```

## 9. Preview updates

After publishing a newer compatible package version:

```sh
mosaic --plain-http deps update --dry-run
```

Apply only after review:

```sh
mosaic --plain-http deps update
mosaic --plain-http deps restore
mosaic validate prod --locked
mosaic test --locked
mosaic build prod --locked
```

## 10. Inspect the result

```sh
mosaic deps graph --format dot > /tmp/dependencies.dot
mosaic cache verify
mosaic inspect prod --resource application.catalog.workload.main
mosaic explain prod application.catalog.workload.main image
```

Mosaic ends with deterministic bundles and provenance. It does not deploy the
rendered resources or contact a cluster.
