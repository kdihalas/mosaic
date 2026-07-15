# Vendoring

`mosaic deps vendor` restores exact locked packages and atomically writes:

```text
vendor/mosaic/
├── vendor.lock
└── packages/<identity>/<version>/
```

Use `mosaic build prod --vendor --offline --locked` to prohibit registry and
unrelated-cache access. Vendored content is rehashed against `mosaic.lock`.
