# Core project workflow

Packages extend Mosaic's existing compiler workflow; they do not replace it.
This chapter provides the project lifecycle needed to use packaged modules in
real environments.

## Create a project

```sh
mosaic init catalog-platform
cd catalog-platform
```

`mosaic.toml` identifies the project and selects source files:

```toml
name = "catalog-platform"
language_version = "v1alpha1"
sources = [
  "modules/**/*.mosaic",
  "applications/**/*.mosaic",
  "variants/**/*.mosaic",
  "policies/**/*.mosaic",
  "environments/**/*.mosaic",
  "tests/**/*.mosaic",
]
exclude = ["dist/**", "vendor/**"]
default_output = "dist"
```

Suggested layout:

```text
mosaic.toml
modules/<module>/types.mosaic
modules/<module>/module.mosaic
applications/<application>.mosaic
variants/<environment-or-purpose>.mosaic
policies/<policy-set>.mosaic
environments/<environment>.mosaic
tests/<behavior>.mosaic
```

Files load deterministically, but filename order does not define semantic
precedence.

## Define a typed module

```mosaic
type ServiceInput {
    name: ResourceName
    image: ImageReference
    owner: string
    replicas: int = 1
}

module Service(input: ServiceInput) {
    serviceAccount "main" {
        name = input.name
    }

    workload "main" {
        name = input.name
        image = input.image
        replicas = input.replicas
        serviceAccount = serviceAccount.main

        labels {
            "app.kubernetes.io/name" = input.name
            "example.com/owner" = input.owner
        }
    }

    expose "http" {
        name = input.name
        workload = workload.main
        ports = [{ name = "http", port = 80, targetPort = 8080 }]
    }

    export workload.main
    export expose.http
    extension workload.main.image
    extension workload.main.replicas
}
```

Use typed resource references such as `serviceAccount.main`, not rendered
resource names. Keep stable local resource identifiers because graph IDs derive
from application alias, kind, and local name.

## Instantiate an application

```mosaic
use Service as catalog {
    name = "catalog-api"
    image = "ghcr.io/acme/catalog-api:1.4.0"
    owner = "catalog-team"
    replicas = 1
}
```

A packaged module changes only the qualified module name:

```mosaic
use http.HttpService as catalog {
    name = "catalog-api"
    image = "ghcr.io/acme/catalog-api:1.4.0"
    owner = "catalog-team"
}
```

The instantiated graph still uses stable resource IDs such as:

```text
application.catalog.workload.main
application.catalog.expose.http
```

## Apply variants

Keep environment-specific mutations outside reusable graph construction:

```mosaic
variant production {
    set catalog.workload.main.replicas = 3

    enable autoscaling {
        target = catalog.workload.main
        minimumReplicas = 3
        maximumReplicas = 12

        cpu {
            averageUtilisation = 65
        }
    }
}
```

Package variants are qualified through the dependency alias:

```mosaic
environment prod {
    use catalog
    apply http.productionDefaults
}
```

Competing writes are not resolved by reordering files or `apply` statements.
Use an explicit environment `resolve` for the conflicting field.

## Enforce policies

```mosaic
policy validContainerImages {
    select Workload {
        deny image.endsWith(":latest") {
            message = "Latest image tags are not permitted."
        }
    }
}
```

Select root or package policies in the environment:

```mosaic
environment prod {
    use catalog
    apply production

    policies {
        use validContainerImages
        use http.requiredResources
    }

    target kubernetes {
        namespace = "catalog-prod"
    }
}
```

Mosaic validates policy results during compilation. It does not contact a
cluster or apply the rendered resources.

## Add configuration tests

```mosaic
test productionUsesStableImage {
    build prod
    assert catalog.workload.main.image ==
        "ghcr.io/acme/catalog-api:1.4.0"
}
```

Run all root tests with exact package content:

```sh
mosaic test --locked
```

Tests embedded in dependency packages remain isolated from the root test run.
Package authors run their tests with `mosaic package validate`.

## Format and parse

```sh
mosaic fmt
mosaic fmt --check
mosaic parse --format tree
```

Use four-space indentation. Quote strings and qualified object keys, but keep
integers, decimals, booleans, and `null` typed rather than quoted.

## Validate and build

```sh
mosaic validate
mosaic validate prod --locked
mosaic build prod --locked
```

Build writes deterministic bundle artifacts under `default_output` unless
`--output` selects a different location. `--clean` replaces an existing build
destination.

For package-backed CI use:

```sh
mosaic validate prod --locked --no-replacements
mosaic test --locked
mosaic build prod --locked
```

## Inspect graph state

Inspect an environment or one resource:

```sh
mosaic inspect prod
mosaic inspect prod \
  --resource application.catalog.workload.main
```

Inspect a compiler phase with `--phase` when diagnosing variants, transforms,
conflicts, references, or policies.

## Explain provenance

Explain a resource or one field:

```sh
mosaic explain prod application.catalog.workload.main
mosaic explain prod application.catalog.workload.main image
```

Provenance identifies module construction, application inputs, variants,
transforms, and environment conflict resolution. Package module ownership uses
the alias-independent package semantic ID.

## Compare bundles

Build before and after a planned change to different directories, then compare
their semantic bundle contents:

```sh
mosaic build prod --locked --output /tmp/prod-before
# Apply and validate the intended source change.
mosaic build prod --locked --output /tmp/prod-after
mosaic diff /tmp/prod-before/bundle.json /tmp/prod-after/bundle.json
```

Use `--fail-on-change` when a semantic difference should produce a CI failure.
Review graph/resource changes, not only rendered YAML text.

## Complete verification loop

For a changed project and environment:

```sh
mosaic fmt
mosaic fmt --check
mosaic parse --format tree
mosaic validate
mosaic validate prod --locked
mosaic test --locked
mosaic build prod --locked
mosaic inspect prod --resource application.catalog.workload.main
mosaic explain prod application.catalog.workload.main image
```

If dependency declarations or local package content changed, run
`mosaic deps resolve` before the locked verification loop and review the
resulting lockfile and dependency graph.
