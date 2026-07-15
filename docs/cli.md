# CLI

Global flags include `--project`, `--format`, `--cache-dir`, and the explicit
OCI test option `--plain-http`. Run `mosaic help <command>` for command flags.

Package commands are `package init|validate|pack|publish|pull|inspect|verify`.
Dependency commands are `deps add|remove|resolve|restore|update|list|graph|vendor`.
Cache commands are `cache list|verify|prune`. Dependency-aware build,
validation, and test commands accept `--locked`, `--offline`, `--vendor`, and
`--update-lock`.

`build` defaults to `dist/<environment>`; YAML/JSON formats can stream one
artifact. `diff --fail-on-change` provides CI gating. `fmt --check` never edits.
No command contacts or mutates a cluster.

Exit codes: 0 success; 1 compilation, validation, policy, formatting, or test
failure; 2 CLI usage; 3 project/filesystem/bundle I/O failure; 4 semantic
differences with `--fail-on-change`.
