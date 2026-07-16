# Conditional construction

`when` conditionally includes declarative construction statements. Its
condition must be a boolean:

```mosaic
when(both(input.monitoring, reverse(empty(input.owner)))) {
    resource "monitor" {
        apiVersion = "monitoring.coreos.com/v1"
        kind = "ServiceMonitor"
        name = input.name
    }

    export resource.monitor
    extension resource.monitor.spec.interval
}
```

Construction guards may read constants and module inputs. They can contain
resources, assignments, named blocks, exports, extensions, protected fields,
and nested `when` statements. An export inside a guard is optional. An
extension or protected declaration inside a guard must target a resource
declared under the same guards.

Variants may use `when` around mutation operations. Every selected variant
guard reads the same base graph snapshot, taken after module instantiation and
before any variant or transform writes:

```mosaic
variant production {
    when(present(catalog.resource.monitor)) {
        set catalog.resource.monitor.spec.interval = "15s"
    }
}
```

`present` accepts an optional exported resource. A successful guard makes that
reference definite within its body. Optional references used without such a
guard are rejected.

Built-in predicates are `gt`, `lt`, `eq`, `includes`, `has`, `empty`, `zero`,
`any`, `both`, `reverse`, and `present`. `any` and `both` accept at least two
booleans and short-circuit from left to right. Existing expression operators
and string methods remain available.

Modules with no `export` statements retain legacy implicit visibility. Once a
module declares an export, all other resources are private. Private resources
are still rendered and selected by policies, but consumers cannot address them
directly.
