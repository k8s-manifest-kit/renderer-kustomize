# Kustomize Renderer Design

## Source model

`kustomize.Source` contains a Kustomization `Path`, an optional
`Values func(context.Context) (map[string]string, error)`, optional
`LoadRestrictions`, and source-specific post-renderers. A source selector can
choose sources at render time.

The renderer supports an on-disk or custom Kustomize filesystem. The default
load restriction is `LoadRestrictionsRootOnly`; a source can override the
renderer-wide setting.

## Virtual values

Resolved values are exposed as a virtual `Path/values.yaml` file in the
filesystem presented to Kustomize. The file is supplied through an in-memory
union overlay created for that render: the renderer never writes to the
underlying filesystem, and the base filesystem is treated as immutable input.
Values are not automatically merged into resources. A Kustomization must
explicitly consume the file using a replacement, generator, or patch.
Render-time values are merged over source values before the virtual file is
created, with render-time keys taking precedence.

Merged values must be scalars: strings, booleans, and real integer, unsigned,
and finite floating-point numbers. Only the built-in types are accepted —
aliases of them included, distinct named types not — so values implementing
`fmt.Stringer` are not treated as numbers or strings. Accepted scalars keep
the default `fmt` representation and are exposed as string data. Null,
composites (maps and slices), pointers, structs, functions, complex numbers,
non-finite floats (NaN and infinities), and any other type cannot be
represented and are rejected at the renderer boundary — before render-time
values are cloned or merged, because the shared clone helpers do not detect
cycles — with an error wrapping `ErrUnsupportedValue` that names the
offending key and its type without including the value. Validation inspects
types only and never recurses into rejected composites, so cyclic values are
rejected by type name instead of being traversed.

Validation runs per selected source, after source selection and before the
render-time clone, cache lookup, and build. Sources removed by a source
selector are never validated and place no scalar constraint on render-time
values, so structured values aimed at another renderer are not rejected by a
deselected Kustomize source. Rejected values never reach a build or the
output cache, so a warm cache (including one configured with a constant
custom key) cannot suppress the error.

## Pipeline and metadata

Kustomize builds each selected source, applies its source-specific
post-renderers, combines the results, then applies renderer-level filters,
transformers, and post-renderers. Source annotations are opt-in. Content hash
annotations use `manifests.k8s-manifests-kit/content.hash` and are enabled by
default.

### Option composition

Renderer options apply left to right. Functional options touch a single
field — `WithFilter`, `WithTransformer`, `WithPostRenderer`,
`WithSourceSelector`, and `WithPlugin` append, and the remaining setters
overwrite only their own field — so they compose with anything around them.
A `RendererOptions` struct is stronger: it replaces the hook, selector,
plugin, load-restriction, warning-handler, and boolean fields outright
(including with nil, the zero value, and false), while nil `CacheOptions`,
`FileSystem`, and plugin-loading pointers preserve what earlier options
configured.

A hook before a struct survives only if the struct carries it; a hook after
a struct is kept:

```go
// The struct clears the filter added before it. Its zero-valued Filters
// and ContentHash replace the previous state, and its zero-valued
// LoadRestrictions (LoadRestrictionsUnknown) discards the constructor's
// RootOnly default — so a struct that wants those defaults must set them
// explicitly.
renderer, _ := kustomize.New(sources,
    kustomize.WithFilter(keepPods),
    kustomize.RendererOptions{
        SourceAnnotations: true,
        ContentHash:       true, // retain the hashing default
        LoadRestrictions:  kustomizetypes.LoadRestrictionsRootOnly, // retain root-only loading
    },
    kustomize.WithPostRenderer(label), // appended to the struct's state
)
```

Because the zero value of `LoadRestrictions` is `LoadRestrictionsUnknown` —
not `LoadRestrictionsRootOnly` — a struct that omits the field falls back to
the source-level restriction, and when both are Unknown kustomize builds
with its unrestricted loader: its `Run` treats every value other than
`RootOnly` as `RestrictionNone`, so resources outside the kustomization root
may be loaded. Set it explicitly when that matters for security. The same
explicitness applies to `ContentHash: true`. These rules
govern the renderer's own options only — the engine's additive options
(`engine.WithFilter` and friends) follow the engine's rules, not these.

### Hash stages

On a cache miss, each built resource is hashed during conversion from
Kustomize's ResMap: after the renderer's source-annotation phase (when
enabled) and before `Run` removes the renderer-added upstream
`config.kubernetes.io/origin` annotation. Only afterwards are the results
stored in the source's output cache (when enabled) and stamped with their
render origin, and the source, renderer, and engine hooks run last — none of
those stages rehash.

When source annotations are enabled and the Kustomization does not already
request `originAnnotations` build metadata, the renderer adds that metadata
so Kustomize stamps upstream origin annotations during the build; those
upstream annotations are still present at hashing time and removed
afterwards, so the stored digest does not match the final returned object in
that case. The cleanup removes `config.kubernetes.io/origin` from every
object unconditionally, so an origin annotation a resource carried for
other reasons is removed too. When the Kustomization requests
`originAnnotations` itself, the renderer injects nothing and removes
nothing: the upstream origins stay on the objects and participate in the
hash.

Consequences callers can rely on: the hash covers the complete object at
hashing time, including any metadata already present. It is not a hash of
the Kustomization's source bytes, and it is not idempotent. The render-origin
stamp and cache retrieval never rehash, and mutations made by later hooks
need not match the stored digest. The full annotation phase contract is the
engine's [annotation contract](https://github.com/k8s-manifest-kit/engine/blob/main/docs/design.md#annotations);
this renderer follows it at the points described above.

## Output cache

Render-result caching is opt-in via `WithCache`. Every registered source
receives its own cache instance configured with the renderer-wide TTL and
key function. Because caches are isolated per source, a source's effective
load restrictions and post-rendering pipeline can never be bypassed through
another source's cached output — including sources that share a path and
values, or a cache configured with a constant custom key. Custom key
functions still receive the public `cache.KustomizationSpec` (path and
values) from `github.com/k8s-manifest-kit/renderer-kustomize/pkg/cache`
(constructed with `cache.New`), and
within a single source a custom key keeps controlling hit behavior.

Cache entries are deep-cloned when stored and when retrieved, so mutating a
rendered result never pollutes later renders. On a miss (or after the
configured TTL, five minutes by default, expires an entry) the source is
rebuilt through the normal Kustomize build, picking up filesystem changes.

## Filesystem support

The public filesystem helpers are in `pkg/fs`: `NewFsOnDisk`,
`NewMemoryFs`, `NewReadOnlyFs`, `NewFromIOFS`, and `NewBasePathFs`. Union
overlays are in `pkg/fs/union` and are created with `union.NewFs` plus
`union.WithOverride`, `union.WithOverrides`, or `union.WithOverlayFs`.

## Remote URL transport ownership

Kustomize resolves remote resources referenced by a Kustomization — for
example, remote component or resource URLs — through its own loader, not
through any client this renderer supplies. At the pinned dependency version,
`sigs.k8s.io/kustomize/api v0.21.3`, the call path is:

- `pkg/kustomize_engine.go` builds a `krusty.Kustomizer` from
  `krusty.Options` and calls `Kustomizer.Run`.
- [`Kustomizer.Run`](https://github.com/kubernetes-sigs/kustomize/blob/api/v0.21.3/api/krusty/kustomizer.go)
  constructs the file loader internally; callers cannot pass one in.
- [`FileLoader.Load`](https://github.com/kubernetes-sigs/kustomize/blob/api/v0.21.3/api/internal/loader/fileloader.go)
  dispatches remote paths to the private `httpClientGetContent` method,
  which falls back to a fresh `&http.Client{}` when the loader's private
  `http` field is unset.

[`krusty.Options`](https://github.com/kubernetes-sigs/kustomize/blob/api/v0.21.3/api/krusty/options.go)
exposes no HTTP client field, so this renderer's public API cannot inject,
replace, or configure the client used for remote fetches. The fallback
`&http.Client{}` has a nil `Transport`, which the standard library resolves
to the shared, process-wide `http.DefaultTransport` — an instance this
repository neither owns nor manages.

Rendering local or in-memory Kustomize sources does not invoke this URL
fetch path. When a Kustomization does reference remote URLs, the fetch runs
on the dependency-owned default construction described above. This is a
known limitation of the pinned upstream version, not a defect in this
renderer, and the shared `pkg/util/httpclient` factories do not reach it.
See [`development.md`](development.md) for the policy that follows from it.

See [`../AGENTS.md`](../AGENTS.md), [`development.md`](development.md), and
[`fs-adapter.md`](fs-adapter.md).
