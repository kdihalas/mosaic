# Package-defined capabilities

Capabilities are declarative package exports that attach optional resources to
an existing application resource. They expand during compilation into ordinary
typed graph resources; packages do not execute code or extend the compiler.

Define a typed input and capability:

```mosaic
type MonitorInput {
    target: reference<Workload>
    name: ResourceName
    enabled: bool = true
    interval: Duration = "30s"
}

capability Monitor(input: MonitorInput) {
    when(input.enabled) {
        resource "monitor" {
            apiVersion = "monitoring.coreos.com/v1"
            kind = "ServiceMonitor"
            name = input.name
            spec {
                endpoints = [{ port = "metrics", interval = input.interval }]
            }
        }

        export resource.monitor
        extension resource.monitor.spec.endpoints
    }
}
```

Export both the capability and every type in its public input:

```toml
[exports]
capabilities = ["Monitor"]
types = ["MonitorInput"]
```

Consumers invoke an exported capability through its dependency alias and give
each invocation an identifier-compatible local name:

```mosaic
variant production {
    enable observability.Monitor as metrics {
        target = catalog.workload.main
        name = "catalog-metrics"
        enabled = true
        interval = "15s"
    }
}
```

Generated resource IDs include the target application and invocation name, for
example `application.catalog.capability.metrics.resource.monitor`. Multiple
capabilities may target one application, but invocation names must be unique
within that application.

Capability bodies follow module construction rules: they may create core or
explicit custom resources, use typed local references, declare exports and
extension/protected fields, and use `when`. Expansion is deterministic and is
subject to the normal graph and resource limits. A capability must take one
parameter named `input`, and every invocation must supply a resource-valued
`target` input.
