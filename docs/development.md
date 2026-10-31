# Kustomize Renderer Development

## Prerequisites and commands

Use the Go version required by [`go.mod`](../go.mod). Run the repository
Makefile targets:

```bash
make test
make test/race
make fmt
make lint
make lint/fix
make check
```

`make check` runs lint and vulncheck; formatting is the separate `make fmt`
target. There is no live integration target — the default test suite is
offline.

## Package layout

- `pkg/kustomize.go` — source processing and render pipeline.
- `pkg/kustomize_engine.go` — Kustomize build and filesystem preparation.
- `pkg/kustomize_support.go` — shared implementation helpers.
- `pkg/kustomize_option.go` — renderer options and defaults.
- `pkg/util/fs/` — filesystem constructors and adapters.
- `pkg/util/fs/union/` — overlay filesystem implementation.

Renderer options are typed by the shared
`github.com/k8s-manifest-kit/pkg/util/option` package. The layout follows the
shared [`project-layout.md`](../../docs/project-layout.md) convention
(workspace navigation).

## Code conventions

- Give each parameter its own explicit type: `name string, namespace string`.
- Keep shared implementation helpers in `*_support.go`; test file layout
  follows the [Test layout](#test-layout) rules below.
- Wrap errors with operation and source context using `%w`, and check
  sentinel errors with `errors.Is`. Validation errors name the path, key, and
  type needed for diagnosis without including the rejected value's contents.
- Follow the formatter and import grouping configured in `.golangci.yml`.
  Renderer tests live in the external `kustomize_test` package and exercise
  the public API; filesystem behavior is tested in its own package.

### Multiline strings

- Use one descriptive, standalone, package-level `const name =` declaration
  with a raw string literal for every multiline string: fixtures, manifests,
  templates, and expected output.
- Never embed multiline string literals in function or method bodies or
  declare them as function-local constants.
- Never group multiline constants in `const (...)`. Related single-line
  scalar constants can still be grouped, and single-line inline strings are
  unaffected.
- Keep large literal content separate from execution logic for readability.
  Preserve meaningful content indentation; the declaration layout must not
  introduce unintended bytes.
- A function may format the named template into a local variable. The
  prohibition concerns where the literal is declared, not where its string
  value is used.

```go
const fixtureYAML = `apiVersion: v1
kind: ConfigMap
`
```

## Testing

- Use Gomega with `t.Context()`. For behavior fixes, add a regression test
  that fails without the fix when practical; assert on fields and identities
  rather than only error counts.
- When changing virtual values, test both the generated `values.yaml` content
  and the Kustomization configuration that consumes it. When changing load
  restrictions, test renderer defaults and per-source overrides. Filesystem
  tests should cover missing files, overlays, read-only behavior, and path
  normalization.
- Ownership tests must actually change the returned object and prove the
  target changed before checking other results or later renders: cached
  results are deep-cloned when stored and retrieved, so mutating one render
  must never affect another.

### Test layout

- Colocate tests with implementation: a `test/` subdirectory is a separate
  Go package, not a cosmetic folder, so use existing package boundaries.
- Keep a cohesive `<component>_test.go` for core API behavior and organize
  larger suites by concern as `<component>_<concern>_test.go` — for example
  `kustomize_cache_test.go`, `kustomize_values_test.go`,
  `kustomize_concurrency_test.go`, or `kustomize_metadata_test.go`. These
  names fit packages that need them; they are not mandatory files.
- Place benchmarks in `<component>_benchmark_test.go` in the same package
  directory, keeping benchmark names and setup/timing behavior intact.
- Put helpers and fixtures shared across a package's tests in
  `<component>_support_test.go`, and substantial feature-specific fixture
  machinery in `<component>_<concern>_support_test.go`. Small test-local
  helpers and scenario assertions remain beside their tests.
- Support files contain helper, fixture, and type declarations only — no
  test, benchmark, fuzz, or example entry points. `_support_test.go` is a
  naming convention; only `_test.go` has Go's test-file significance.
- Keep tests discoverable without excessive fragmentation: no hard line
  limit, no one-file-per-test rule, no shared cross-repository harness, and
  no fixture-storage migration. There is no live integration target, and
  filenames alone never exclude a test from the default offline suite.

## Concurrency, caching, and filesystems

`make test/race` re-runs the unit suite under the race detector. The
concurrency tests render overlapping requests with distinct scalar values
against a single shared renderer, so run it whenever changing the virtual
values overlay, cache handling, source isolation, or output cloning.

- Overlapping `Process` calls on one shared renderer are safe: the virtual
  `values.yaml` overlay is created for that render and never written to the
  base filesystem, which is treated as immutable input. Use the read-only
  and union adapters in `pkg/util/fs` for custom filesystems.
- User-supplied code — `Values` functions, source selectors, cache key
  functions, KRM and builtin plugins, and post-renderers — runs inside
  concurrent renders and must itself be safe for concurrent use; the
  renderer does not synchronize arbitrary callbacks.
- Caches are isolated per source: a source's effective load restrictions and
  post-rendering pipeline cannot be bypassed through another source's cached
  output, even when the cache is configured with a constant custom key.
  Custom key functions still control hit behavior within a single source.
- Value validation runs after source selection and before the render-time
  clone, cache lookup, and build: deselected sources place no scalar
  constraint on render-time values.

Regression anchors for these boundaries: `TestRenderTimeValueValidation` in
`pkg/kustomize_values_test.go` (selected-source ordering, cyclic composites,
error privacy), `TestCacheSourceIsolation` in `pkg/kustomize_cache_test.go`,
and `TestConcurrentRendering` in `pkg/kustomize_concurrency_test.go`.

See [`design.md`](design.md), [`fs-adapter.md`](fs-adapter.md), and
[`../AGENTS.md`](../AGENTS.md).
