# OCI distribution

Mosaic uses standard OCI registries with these media types:

- `application/vnd.mosaic.package.manifest.v1+json` artifact type
- `application/vnd.mosaic.package.config.v1+json` config
- `application/vnd.mosaic.package.layer.v1.tar+gzip` archive layer

Publishing creates an immutable semantic-version tag and prints its manifest
digest. Mutable aliases are explicit. Authentication uses a public credential
provider; the CLI adapts Docker credential configuration. Plain HTTP requires
`--plain-http`.
