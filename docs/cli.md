# CLI

Global flags are `--project`, `--format`, `--no-color`, `--max-diagnostics`,
`--verbose`, and `--quiet`. Run `mosaic help <command>` for command flags.

`build` defaults to `dist/<environment>`; YAML/JSON formats can stream one
artifact. `diff --fail-on-change` provides CI gating. `fmt --check` never edits.
No command contacts or mutates a cluster.

Exit codes: 0 success; 1 compilation, validation, policy, formatting, or test
failure; 2 CLI usage; 3 project/filesystem/bundle I/O failure; 4 semantic
differences with `--fail-on-change`.
