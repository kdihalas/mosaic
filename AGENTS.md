# Instructions for agents generating Mosaic configuration

Use these instructions whenever a task asks you to create or modify Mosaic
configuration in this repository.

## Goal and boundaries

Produce deterministic `.mosaic` source that passes formatting, validation, and
configuration tests and builds the requested environments. Mosaic produces
bundles; it does not deploy them.

Never add Kubernetes clients, `kubectl` calls, apply/deploy steps, controllers,
reconciliation loops, cluster reads, shell execution, environment-variable
lookups, network calls, current-time values, or random values to Mosaic source.
Do not edit generated `dist/` files directly.

## Start by discovering the project

1. Find the nearest `mosaic.toml`. Treat its directory as the project root.
2. Read its `sources`, `exclude`, and `default_output` settings.
3. Inspect existing declarations before adding new ones:

   ```sh
   mosaic --project <root> parse --format tree
   mosaic --project <root> validate
   ```

4. Search existing modules, types, applications, variants, environments,
   policies, and tests. Reuse an existing module when its exported resources
   and extension points satisfy the task.
5. Never infer precedence from filenames. Files are loaded lexicographically,
   but file order has no semantic precedence.

## Recommended source layout

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

Keep reusable graph construction in modules, deployment-specific changes in
variants, target selection in environments, and invariants in policies/tests.

## Authoring rules

- Use four-space indentation and run `mosaic fmt` rather than formatting by
  hand.
- Quote strings. Do not quote integers, decimals, booleans, or `null`:

  ```mosaic
  replicas = 3
  enabled = true
  image = "ghcr.io/acme/catalog-api:1.4.0"
  memory = "256Mi"
  ```

- Quote object keys containing `/`, `.`, or other punctuation:

  ```mosaic
  labels {
      "app.kubernetes.io/name" = input.name
      "something.com/owner" = input.owner
  }
  ```

- Use typed resource references, not rendered names:

  ```mosaic
  serviceAccount = serviceAccount.main
  workload = workload.main
  ```

- Give every resource a stable local name. Its graph identity becomes
  `application.<application-alias>.<resource-kind>.<local-name>` and must not
  depend on a filename or rendered Kubernetes name.
- Use only the supported backend-neutral resource declarations: `config`,
  `serviceAccount`, `workload`, and `expose`. Enable `autoscaling` and
  `disruptionProtection` as capabilities in variants.
- Supply every module input referenced by the module. Prefer block input syntax
  for nested objects, but never use both block and assignment forms for one
  field.
- Export resources that consumers may inspect. Declare intentional mutation
  points with `extension` and security-sensitive fields with `protected`.
- Do not resolve competing writes by reordering files or `apply` statements.
  Add an explicit environment `resolve` for the specific conflicting field.

## Minimal pattern

Define the input and module:

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
            "something.com/owner" = input.owner
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

Instantiate it once in an application:

```mosaic
use Service as catalog {
    name = "catalog-api"
    image = "ghcr.io/acme/catalog-api:1.4.0"
    owner = "catalog-team"
    replicas = 1
}
```

Express environment changes in a variant:

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

Compose a complete build target in an environment:

```mosaic
environment prod {
    use catalog
    apply production

    policies {
        use validContainerImages
    }

    target kubernetes {
        namespace = "catalog-prod"
    }
}
```

Protect invariants with a policy and a test:

```mosaic
policy validContainerImages {
    select Workload {
        deny image.endsWith(":latest") {
            message = "Latest image tags are not permitted."
        }
    }
}

test productionUsesStableImage {
    build prod
    assert catalog.workload.main.image ==
        "ghcr.io/acme/catalog-api:1.4.0"
}
```

## Required verification

After changing configuration, run all of the following from any directory by
passing the project root explicitly:

```sh
mosaic --project <root> fmt
mosaic --project <root> fmt --check
mosaic --project <root> parse --format tree
mosaic --project <root> validate
mosaic --project <root> validate <each-changed-environment>
mosaic --project <root> test
mosaic --project <root> build <each-changed-environment>
```

Then inspect the affected resource and provenance when relevant:

```sh
mosaic --project <root> inspect <environment> \
  --resource application.<alias>.<kind>.<name>

mosaic --project <root> explain <environment> \
  application.<alias>.<kind>.<name> <field-path>
```

For changes to an existing environment, build before and after bundles and run
`mosaic diff <old-bundle> <new-bundle>`. Review semantic changes, not only YAML.

## Completion checklist

- All source is formatted and parses without diagnostics.
- Every changed environment validates, builds, and selects the intended
  application instances, variants, and policies.
- `mosaic test` passes.
- Resource IDs and references are stable and meaningful.
- Numeric values remain numeric; quantities such as `100m` and `256Mi` remain
  strings.
- Qualified labels and annotations are quoted and valid.
- Rendered resource counts and semantic diffs match the request.
- No generated artifact contains timestamps, absolute paths, random IDs, or
  host-specific data.
- No deployment or cluster-management behavior was introduced.

Use `examples/catalog-platform` as the canonical working reference.
