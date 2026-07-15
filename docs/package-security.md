# Package security

Packages are data, not executable extensions. Mosaic does not run scripts,
plugins, hooks, shell commands, or package-supplied network operations.

Archives reject traversal, absolute paths, links, devices, sockets, pipes,
duplicates, case collisions, and content over configured limits. Extraction
verifies identity, version, inventory, and every digest before an atomic
rename. Offline mode does not initialize registry clients or credentials.
