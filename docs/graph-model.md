# Graph model

The graph stores typed resources, reference edges, labels, annotations, source
metadata, and immutable typed fields. All mutations use graph/transform APIs.
Resources sort by stable ID and fields serialize with sorted keys.

Field paths are dot-separated segments. Capabilities use IDs such as
`application.catalog.capability.autoscaling` and are ordinary inspectable graph
resources. Provenance records deterministic sequence numbers without time.
