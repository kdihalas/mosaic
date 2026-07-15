# CLI reference

This chapter summarizes the package-related CLI. Run `mosaic help` or
`mosaic <group> <command> --help` for generated command help.

## Global options

| Option | Purpose |
| --- | --- |
| `--project <path>` | Project root; default is `.`. |
| `--format text|json` | Human or machine-readable output. |
| `--cache-dir <path>` | Override package cache location. |
| `--plain-http` | Permit insecure OCI HTTP, intended for local testing. |
| `--max-diagnostics <n>` | Bound emitted diagnostics. |
| `--no-color` | Disable color output. |
| `--quiet` | Suppress successful summaries. |
| `--verbose` | Request verbose output where supported. |

Global flags precede or follow subcommands according to Cobra parsing, but
placing them before the command is clearest:

```sh
mosaic --project ./catalog --format json deps list
```

## Package commands

### `package init [directory]`

Creates a complete, validated example package without overwriting existing
files.

### `package validate [path]`

Validates manifest, source, exports, compilation, and tests.

Options: `--offline`, `--locked`.

### `package pack [path]`

Creates a deterministic `.mosaicpkg`. Default output is a package-derived file
under `dist`.

Options: `--output <path>`, `--skip-tests`, `--offline`, `--locked`.

### `package publish [path]`

Validates, tests, packs, and publishes an OCI artifact.

Options:

- `--reference <oci-reference>` for an exact destination;
- `--registry <oci-repository>` to use the manifest version tag;
- repeated `--tag <tag>` for explicit aliases;
- `--allow-tag-move` for explicitly mutable additional tags.

### `package pull <reference>`

Pulls, verifies, and caches an OCI package.

Options: `--output <path>`, `--cache-only`.

### `package inspect <reference-or-path>`

Prints package or artifact metadata for a directory, archive, or OCI reference.
The current command emits JSON metadata.

### `package verify <path-or-reference>`

Verifies a directory, archive, or OCI package without executing code.

## Dependency commands

### `deps add <alias> <source> [constraint]`

Adds and resolves a dependency transactionally. OCI sources require a
constraint; local paths do not accept one.

### `deps remove <alias>`

Removes a dependency and re-resolves transactionally.

### `deps resolve`

Resolves declarations and writes `mosaic.lock`.

Options: `--offline`, `--include-prerelease`.

### `deps restore`

Restores exact locked archives without changing selected versions.

Options: `--offline`, `--vendor`.

### `deps update [aliases...]`

Previews and optionally applies newer versions satisfying declared constraints.

Options: `--dry-run`, `--include-prerelease`. `--major` currently returns an
unsupported-operation error rather than widening constraints.

### `deps list`

Lists direct and transitive locked packages. Supports global JSON output.

### `deps graph`

Prints the locked graph. Options: `--format tree|json|dot`.

### `deps vendor`

Writes the verified vendor tree. Options: `--output <path>`, `--offline`.

## Cache commands

### `cache list`

Lists cache metadata and verification state.

### `cache verify`

Verifies every discovered cache entry and exits unsuccessfully if any is
corrupt.

### `cache prune`

Removes unprotected entries.

Options: `--dry-run`, `--project-lockfiles <paths>`. `--older-than` is accepted
but not currently enforced.

## Dependency-aware compiler commands

`validate`, `build`, and `test` accept:

| Flag | Behavior |
| --- | --- |
| `--locked` | Never modify the lock; fail if missing or stale. |
| `--offline` | Use local and verified cached content without registry access. |
| `--vendor` | Use the project vendor tree. |
| `--update-lock` | Resolve and write the lock before continuing. |

`validate` additionally supports `--no-replacements`.

## Exit behavior

CLI usage errors return exit code `2`. Project loading and filesystem setup
errors generally return `3`; compilation, validation, integrity, and package
operation failures return a nonzero operation status. Automation should prefer
structured diagnostics and success/failure rather than depending on undocumented
subcategories of exit codes.
