# Agent Guide: renderer-kustomize

`renderer-kustomize` renders Kustomize directories through the Kustomize v0.21 API and returns Kubernetes `unstructured.Unstructured` objects.

## Documentation

- [README](README.md) — user-facing overview.
- [Design](docs/design.md) — rendering and option behavior.
- [Filesystem adapters](docs/fs-adapter.md) — custom, embedded, read-only, and union filesystems.
- [Development](docs/development.md) — workflow and tests.

## Public API

The package is imported from `github.com/k8s-manifest-kit/renderer-kustomize/pkg`.

- `kustomize.New([]kustomize.Source{...}, opts...)` creates a renderer.
- `kustomize.NewEngine(source, opts...)` creates an `engine.Engine` for one source.
- `Source` contains `Path`, an optional dynamic `Values` function, per-source `LoadRestrictions`, and source-specific `PostRenderers`.
- `Values(map[string]string)` provides static source values.
- Options include filters, transformers, post-renderers, source selectors, caching, source annotations, content hashes, load restrictions, warning handlers, custom filesystems, KRM/builtin plugin loading, and managed-by labels.

Values are exposed through a virtual `values.yaml` overlay and are not applied automatically. The Kustomization must explicitly consume them through its own generators, replacements, or patches. Source annotations use the shared source type, path, and file constants.

The default load restriction is `LoadRestrictionsRootOnly`. Function plugins require explicit plugin-loading options and open Kustomize plugin restrictions as documented by the option comments.

## Filesystems

Use `pkg/util/fs` constructors such as `NewFsOnDisk`, `NewMemoryFs`, `NewReadOnlyFs`, `NewFromIOFS`, and `NewBasePathFs`. Union overlays are in `pkg/util/fs/union` and use `union.NewFs` with `WithOverride`, `WithOverrides`, or `WithOverlayFs`.

## Development

Run commands from this directory:

```bash
make test
make fmt
make lint
make lint/fix
make check
```

Use the checked-in fixtures under `config/test/kustomizations`, test filesystem behavior in its package, and use `t.Context()` and Gomega in renderer tests.
