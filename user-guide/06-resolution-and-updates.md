# Dependency resolution and updates

## Resolution model

Mosaic uses one version per package identity in a dependency graph. It collects
every constraint on an identity and deterministically selects the highest
stable version satisfying all constraints.

For example:

```text
catalog-platform requires acme/observability ^2.2.0
acme/http-service@1.4.0 requires acme/observability >=2.3.0 <3.0.0
```

If `2.6.1` is the highest available stable version satisfying both, Mosaic
selects `2.6.1`. Candidate versions are ordered semantically, never
lexicographically.

Resolution is independent of TOML declaration order, source-file order, map
iteration order, and registry tag response order.

## Resolve a project

```sh
mosaic deps resolve
```

This command may query OCI tag and manifest metadata but does not need to
restore every full archive. It writes a deterministic `mosaic.lock`.

Prefer an already locked compatible version while resolving:

```sh
mosaic deps resolve
```

The CLI enables locked-version preference by default for explicit resolution.
Use an update command when newer compatible versions should be considered.

## Constraints

| Constraint | Meaning |
| --- | --- |
| `1.4.0` | Exactly `1.4.0`. |
| `^1.4.0` | Compatible `1.x` release, at least `1.4.0`. |
| `~1.4.0` | Patch releases in the `1.4` line. |
| `>=1.4.0` | Any version at least `1.4.0`. |
| `>=1.4.0 <2.0.0` | Intersection of two comparisons. |
| `1.x` | Any `1.x` release. |
| `1.4.x` | Any `1.4.x` release. |

Invalid constraints produce `DEP002` structured diagnostics.

## Prereleases

Prereleases are excluded unless:

- the constraint itself contains a prerelease; or
- `--include-prerelease` is explicitly supplied.

```sh
mosaic deps resolve --include-prerelease
mosaic deps update --include-prerelease --dry-run
```

Use prerelease opt-in consistently in CI if a project intentionally depends on
prerelease packages.

## Update dependencies safely

Preview compatible changes:

```sh
mosaic deps update --dry-run
```

Preview selected aliases:

```sh
mosaic deps update http observability --dry-run
```

Apply compatible updates:

```sh
mosaic deps update
mosaic deps restore
mosaic validate --locked
mosaic test --locked
```

Updates preserve declared constraints. `--major` constraint widening is not
implemented; edit the intended constraint explicitly, review it, and run
`mosaic deps resolve`.

## Conflict diagnostics

When no version satisfies every requirement, Mosaic emits `DEP012` and reports
the contributing requirement paths. It never silently chooses one constraint
over another.

Resolve conflicts by one of these explicit actions:

- widen a root constraint after reviewing compatibility;
- update a package whose transitive constraint is too narrow;
- keep the locked compatible version;
- use an explicit local replacement while developing a coordinated change.

Do not rely on dependency declaration ordering; it has no resolution effect.

## Cycles and source conflicts

Package cycles fail with `DEP015` before compiler source loading. Two effective
sources claiming the same identity fail with `DEP020` unless a single explicit
replacement defines the effective source.

These checks prevent ambiguous ownership and dependency-confusion behavior.

## Offline resolution

```sh
mosaic deps resolve --offline
```

Offline resolution uses local packages and verified cache metadata only. It
does not initialize an OCI client, retrieve registry credentials, resolve DNS,
or fall back to the network. It can resolve only versions already represented
locally.
