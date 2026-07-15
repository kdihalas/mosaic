# Mosaic

Mosaic is a deterministic typed configuration compiler. It composes reusable
modules, explicit environment variants, policies, references, and provenance
into backend-neutral graphs and validated Kubernetes bundles. It is a compiler
and library toolkit—not a controller, deployer, reconciler, or cluster client.

## Install and quick start

```sh
go install github.com/kdihalas/mosaic/cmd/mosaic@latest
mosaic init catalog-platform
cd catalog-platform
mosaic validate
mosaic build dev
mosaic build stage
mosaic build prod
```

The catalog example renders ConfigMap, ServiceAccount, Service, and Deployment
for dev; stage also includes an HPA; prod also includes a PDB.

For custom-resource composition, [examples/operator-integrations](examples/operator-integrations)
builds cert-manager `Certificate`, Prometheus Operator `ServiceMonitor`, External
Secrets Operator `ExternalSecret`, and Argo Rollouts `Rollout` resources. The
example includes typed inputs, variants, policies, tests, and deterministic
dev/prod bundles.

Commands are `init`, `fmt`, `parse`, `validate`, `build`, `inspect`, `explain`,
`diff`, `test`, `version`, and the developer-oriented `lex`. See
[docs/cli.md](docs/cli.md).

Coding agents that generate Mosaic configuration should follow
[AGENTS.md](AGENTS.md), which includes project discovery, authoring patterns,
type-preservation rules, and the required verification workflow.

## Library use

```go
p, ds := project.Load(ctx, "./catalog-platform", project.LoadOptions{})
c := compiler.New(compiler.NewOptions{})
r, ds := c.Compile(ctx, p, compiler.Options{Environment: "prod"})
a, ds := kubernetes.New().Render(ctx, renderer.RenderInput{
    Environment: r.Environment, Graph: r.Graph,
    Provenance: r.Provenance, Options: r.Metadata.TargetOptions,
})
b, err := bundle.Build(r, *a, bundle.BuildOptions{})
```

The language, bundle format, and compiler are currently `v1alpha1`,
`v1alpha1`, and `0.1.0` respectively.
