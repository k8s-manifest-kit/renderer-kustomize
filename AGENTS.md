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

Render-time values passed to `Process` — including through `engine.Render` with `render.WithValues` — must be scalars: strings, booleans, or real integer, unsigned, or finite floating-point numbers. Only the built-in types are accepted — aliases of them included, distinct named types not — so values implementing `fmt.Stringer` are not treated as strings. Merged values are exposed as string data in the virtual `values.yaml` ConfigMap using the default `fmt` representation. Null, maps, slices, pointers, structs, functions, complex numbers, non-finite floats, and any other type are rejected before rendering or cache lookup with an error wrapping `ErrUnsupportedValue` that names the offending key and its type without including the value. Validation runs per source after source selection, so sources removed by a selector do not constrain render-time values — structured values consumed by another renderer are not rejected by a skipped Kustomize source.

The default load restriction is `LoadRestrictionsRootOnly`. Function plugins require explicit plugin-loading options and open Kustomize plugin restrictions as documented by the option comments.

## Filesystems

Use `pkg/fs` constructors such as `NewFsOnDisk`, `NewMemoryFs`, `NewReadOnlyFs`, `NewFromIOFS`, and `NewBasePathFs`. Union overlays are in `pkg/fs/union` and use `union.NewFs` with `WithOverride`, `WithOverrides`, or `WithOverlayFs`.

## Development

Run commands from this directory:

```bash
make test
make test/race
make fmt
make lint
make lint/fix
make check
```

Use the checked-in fixtures under `config/test/kustomizations`, test filesystem behavior in its package, and use `t.Context()` and Gomega in renderer tests. Keep tests focused and simple: use the smallest local fixture that exercises the behavior, and add helpers or extra test files only when a concrete behavior or regression requires them.
