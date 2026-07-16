# Bundle format v1alpha1

A bundle directory contains `bundle.json`, `graph.json`, `schema.json`,
`extension-points.json`, `provenance.json`, `policy-report.json`,
`kubernetes.yaml`, and `kubernetes.json`. Composable bundles also contain
`build-recipe.json`, a path-independent snapshot of the verified source units
needed to recompile baked-in variants without registry or filesystem access.

`schema.json` inventories graph resource IDs and types plus instantiated module
exports. Optional exports record optionality and whether the resource is
present in the built environment.
`extension-points.json` records the fields that variants may change and the
protected fields they must not change. Bundle derivation recompiles only the
source embedded in `build-recipe.json`; it does not resolve new dependencies.

JSON is canonical and YAML object order is ServiceAccount, ConfigMap, Service,
Deployment, HPA, then PDB; ties use namespace, name, and resource ID.

Each non-manifest file has a SHA-256 digest. The graph digest covers canonical
graph bytes. The bundle digest covers a canonical manifest with its own digest
empty. Writes stage all files in a sibling directory before replacement.
