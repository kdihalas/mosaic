# Type system

Primitive types are `null`, `bool`, `int`, `decimal`, and `string`. Compound
types are `list<T>`, `map<K,V>`, objects, enums, `optional<T>`, `resource<K>`,
and `reference<K>`. Decimal values use exact rational arithmetic.

Declared fields may have defaults and `require` constraints. Enum members are
resolved using their expected type. Domain scalars are `ImageReference`,
`CpuQuantity`, `MemoryQuantity`, `Duration`, `PortNumber`, and `ResourceName`.
Generic graph names remain backend neutral; Kubernetes restrictions are checked
by the renderer.
