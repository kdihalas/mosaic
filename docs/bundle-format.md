# Bundle format v1alpha1

A bundle directory contains `bundle.json`, `graph.json`, `provenance.json`,
`policy-report.json`, `kubernetes.yaml`, and `kubernetes.json`. JSON is
canonical and YAML object order is ServiceAccount, ConfigMap, Service,
Deployment, HPA, then PDB; ties use namespace, name, and resource ID.

Each non-manifest file has a SHA-256 digest. The graph digest covers canonical
graph bytes. The bundle digest covers a canonical manifest with its own digest
empty. Writes stage all files in a sibling directory before replacement.
