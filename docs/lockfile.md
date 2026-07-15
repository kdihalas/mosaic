# Lockfile

`mosaic.lock` binds identity, exact version, source, aliases, dependency edges,
and manifest, content, archive, and OCI digests. Entries are deterministic and
contain no credentials, timestamps, or absolute cache paths.

Dependency-bearing builds require a current lockfile. `--locked` prohibits
updates; `--update-lock` resolves explicitly. Changed constraints, sources,
replacements, or local content fail with a diagnostic recommending
`mosaic deps resolve`.
