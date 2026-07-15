# Architecture

Mosaic runs lex → parse → resolve names → check types → instantiate modules →
build graph → apply variants/transforms → detect/resolve conflicts → resolve
references → validate graph → evaluate policies. Each phase returns structured
data and diagnostics; graph phases expose immutable snapshots.

Reusable functionality lives under `pkg`. The CLI only loads flags/files,
calls public packages, renders diagnostics, and selects an exit code. Values
and graph storage are encapsulated and canonical. Capabilities are graph
resources. Renderers consume a graph without mutating it.

There is intentionally no Kubernetes client, controller loop, CRD, admission
webhook, release store, or reconciliation behavior. Another product may import
the graph and bundle packages to implement those concerns.

Package resolution precedes compilation. The manifest and lockfile select
exact content, restoration verifies it, and callers supply explicit compilation
packages. The compiler has no registry, credential, cache, or network
dependency. Each package owns a symbol table; only exports are visible through
direct dependency aliases.
