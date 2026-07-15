# Public Go APIs

The CLI is a thin adapter. Applications can resolve, lock, restore, and compile
packages without invoking a subprocess.

## Load a project and configure sources

```go
package main

import (
    "context"
    "fmt"

    "github.com/kdihalas/mosaic/pkg/dependency"
    "github.com/kdihalas/mosaic/pkg/packagefs"
    "github.com/kdihalas/mosaic/pkg/project"
    "github.com/kdihalas/mosaic/pkg/registry/oci"
)

func main() {
    ctx := context.Background()
    cfg, _, ds := project.LoadManifest(
        "./catalog-platform",
        project.LoadOptions{},
    )
    if ds.HasErrors() {
        panic(ds)
    }

    local := packagefs.NewLocalResolver(packagefs.Options{
        ProjectRoot: "./catalog-platform",
        AllowExternal: cfg.AllowExternalLocalDependencies,
    })
    remote := oci.New(oci.Options{})

    resolver := dependency.NewResolver(local, remote)
    resolution, ds := resolver.Resolve(ctx, dependency.ResolveInput{
        Project: cfg,
        ProjectRoot: "./catalog-platform",
        Sources: []dependency.SourceResolver{local, remote},
        Options: dependency.ResolveOptions{PreferLocked: true},
    })
    if ds.HasErrors() {
        panic(ds)
    }
    fmt.Println(len(resolution.Packages))
}
```

Diagnostics are returned as `diagnostics.List`; libraries do not print or exit.

## Write a deterministic lockfile

```go
locked := resolution.Lock(cfg)
if err := lockfile.Write(
    "./catalog-platform/mosaic.lock",
    locked,
); err != nil {
    panic(err)
}
```

Use `lockfile.Read`, `ValidateStructure`, and `ValidateProject` when accepting a
lockfile supplied by another component.

## Restore exact packages

```go
cache, err := packagecache.New(packagecache.Options{
    Root: "./cache",
})
if err != nil {
    panic(err)
}

restorer := packagecache.NewRestorer(
    cache,
    packagecache.RestoreOptions{
        Sources: []packagecache.ArchiveSource{local, remote},
    },
)

report, ds := restorer.Restore(ctx, locked)
if ds.HasErrors() {
    panic(ds)
}
fmt.Printf("restored %d packages\n", report.Restored)
```

Set `RestoreOptions.Offline` to prohibit source fetching. Supply only resolvers
appropriate for the calling application's trust model.

## Explicit credentials and HTTP behavior

```go
credentials := registry.CredentialProviderFunc(
    func(ctx context.Context, host string) (registry.Credentials, error) {
        return credentialStore.Lookup(ctx, host)
    },
)

remote := oci.New(oci.Options{
    Credentials: credentials,
    HTTPClient: customHTTPClient,
    Limits: packageLimits,
})
```

`oci.NewDockerCredentials` is available for CLI-like Docker configuration
behavior. Library consumers are not required to use global credential state.
Set `PlainHTTP` only for an intentional local test registry.

## Pack and verify archives

```go
artifact, ds := packagearchive.Pack(
    ctx,
    "./packages/http-service",
    packagearchive.PackOptions{Limits: packageLimits},
)
if ds.HasErrors() {
    panic(ds)
}

verified, ds := packagearchive.Verify(
    ctx,
    artifact.Bytes,
    packagearchive.VerifyOptions{
        Limits: packageLimits,
        ExpectedArchiveDigest: artifact.ArchiveDigest,
        ExpectedContentDigest: artifact.ContentDigest,
    },
)
```

Use `packagearchive.Unpack` rather than a generic tar extractor so Mosaic's
path, file-type, inventory, and size protections are applied.

## Compile resolved package source

The compiler never performs restoration or registry access. Convert verified
cache or vendor packages into explicit compiler units:

```go
root, ds := project.Load(
    ctx,
    "./catalog-platform",
    project.LoadOptions{},
)
if ds.HasErrors() {
    panic(ds)
}

input := compiler.Input{
    RootProject: root,
    Packages: []compiler.CompilationPackage{
        {
            Identity: packageIdentity,
            Version: packageVersion,
            Aliases: []string{"http"},
            Root: loadedPackage.Root,
            Files: loadedPackage.Files,
            Manifest: loadedPackage.Manifest,
            Exports: loadedPackage.Manifest.Exports,
        },
    },
    Environment: "prod",
}

result, ds := compiler.New(compiler.NewOptions{}).CompileInput(ctx, input)
if ds.HasErrors() {
    panic(ds)
}
```

Populate package dependency metadata when constructing nested package
namespaces. Stable symbol ownership derives from identity and version, not the
root alias.

## Configure safety limits

```go
limits := mosaicpackage.DefaultLimits()
limits.MaxArchiveBytes = 64 << 20
limits.MaxFiles = 10_000
limits.MaxDependencies = 200
```

Pass the same policy to archive, source, cache, resolver, and OCI components.
Zero-valued fields are filled with defaults.

## API packages

| Package | Role |
| --- | --- |
| `pkg/package` | Manifest, identity, version, exports, validation, limits. |
| `pkg/dependency` | Constraints, resolver interfaces, graph, resolution. |
| `pkg/lockfile` | Model, deterministic I/O, validation. |
| `pkg/packagearchive` | Deterministic pack, verify, and safe unpack. |
| `pkg/packagefs` | Sandboxed local source resolver. |
| `pkg/packagecache` | Content cache and exact restoration. |
| `pkg/registry` | Registry-neutral credentials and publishing contracts. |
| `pkg/registry/oci` | OCI source resolver, pull, and publish client. |
| `pkg/vendor` | Vendor tree creation, verification, and loading. |
| `pkg/compiler` | Explicit compilation packages and compiler pipeline. |
