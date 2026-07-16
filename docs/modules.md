# Modules

A module accepts one typed object parameter and produces resources in a local
namespace. `export` controls visibility, `extension` allows external mutation,
and `protected` rejects it. A top-level `use Module as alias` supplies input.

Modules without any export statements retain implicit visibility for
compatibility. Otherwise, only explicit exports are public. An export declared
inside `when` is optional; its resource must be guarded with `present` before a
variant addresses it.

Resource IDs are independent of files and rendered names:
`application.<alias>.<kind>.<local-name>`. References remain typed graph values
until a renderer translates them.
