# Policies

Policies run after transforms, conflicts, and references, before rendering.
Selectors can filter resource kinds and use a deterministic expression engine
with access, comparisons, logic, nulls, capability lookup, list size, and
`contains`, `startsWith`, `endsWith`, and `matches`.

Policy evaluation cannot access files, environment variables, the network,
time, randomness, or processes. Reports sort by policy, resource, and rule.
