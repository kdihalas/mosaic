# Transforms and conflicts

Defaults and module construction establish the base graph. Applied variants and
transforms are peer owners: their textual or application order creates no
precedence. Equal writes and non-overlapping object/list identities coalesce;
different scalar/key/identity writes and delete/update pairs conflict.

`resolve <resource>.<field> = <value>` is the sole higher-precedence environment
operation. It resolves only its addressed ambiguity and is recorded in
provenance.
