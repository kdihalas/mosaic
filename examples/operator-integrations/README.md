# Operator integrations

This advanced Mosaic project renders custom resources for four well-known
Kubernetes operators:

- [cert-manager Certificate](https://cert-manager.io/docs/usage/certificate/)
- [Prometheus Operator ServiceMonitor](https://prometheus-operator.dev/docs/developer/getting-started/)
- [External Secrets Operator ExternalSecret](https://external-secrets.io/latest/api/externalsecret/)
- [Argo Rollouts Rollout](https://argoproj.github.io/argo-rollouts/features/specification/)

The corresponding CRDs and controllers must already be installed before these
artifacts can be used by a separate deployment product. Mosaic only validates,
composes, and renders the resources; it does not install operators or contact a
cluster.

```sh
go run ./cmd/mosaic --project ./examples/operator-integrations validate
go run ./cmd/mosaic --project ./examples/operator-integrations test
go run ./cmd/mosaic --project ./examples/operator-integrations build prod
```
