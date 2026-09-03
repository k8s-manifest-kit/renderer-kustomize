# Kustomize Renderer Development

## Prerequisites and commands

Use Go 1.26.8. Run the repository Makefile targets:

```bash
make test
make fmt
make lint
make lint/fix
make check
```

## Package layout

- `pkg/kustomize.go` — source processing and render pipeline.
- `pkg/kustomize_engine.go` — Kustomize build and filesystem preparation.
- `pkg/kustomize_option.go` — renderer options and defaults.
- `pkg/util/fs/` — filesystem constructors and adapters.
- `pkg/util/fs/union/` — overlay filesystem implementation.

When changing virtual values, test both the generated `values.yaml` content
and the Kustomization configuration that consumes it. When changing load
restrictions, test renderer defaults and per-source overrides. Filesystem
tests should cover missing files, overlays, read-only behavior, and path
normalization.

See [`design.md`](design.md), [`fs-adapter.md`](fs-adapter.md), and
[`../AGENTS.md`](../AGENTS.md).
