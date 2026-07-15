# Package manifest

Every package contains `mosaic.package.toml`:

```toml
name = "acme/http-service"
version = "1.4.0"
language_version = "v1alpha1"
sources = ["src/**/*.mosaic", "tests/**/*.mosaic"]
exclude = ["dist/**", ".git/**"]

[exports]
modules = ["HttpService"]
types = ["HttpServiceInput"]
policies = ["requiredResources"]

[dependencies]
observability = { source = "oci://ghcr.io/acme/mosaic/observability", version = "^2.0.0" }
```

Names are registry-independent lowercase slash-separated identities. Manifest
versions are exact semantic versions. Exports are checked against parsed
declarations, including the complete public type closure.
