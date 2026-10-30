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

Resolved values are written as a virtual `Path/values.yaml` file in the
filesystem presented to Kustomize. Values are not automatically merged into
resources. A Kustomization must explicitly consume the file using a
replacement, generator, or patch. Render-time values are merged over source
values before the virtual file is created.

## Pipeline and metadata

Kustomize builds each selected source, applies its source-specific
post-renderers, combines the results, then applies renderer-level filters,
transformers, and post-renderers. Source annotations are opt-in. Content hash
annotations use `manifests.k8s-manifests-kit/content.hash` and are enabled by
default.

## Output cache

Render-result caching is opt-in via `WithCache`. Every registered source
receives its own cache instance configured with the renderer-wide TTL and
key function. Because caches are isolated per source, a source's effective
load restrictions and post-rendering pipeline can never be bypassed through
another source's cached output — including sources that share a path and
values, or a cache configured with a constant custom key. Custom key
functions still receive the public `KustomizationSpec` (path and values), and
within a single source a custom key keeps controlling hit behavior.

Cache entries are deep-cloned when stored and when retrieved, so mutating a
rendered result never pollutes later renders. On a miss (or after the
configured TTL, five minutes by default, expires an entry) the source is
rebuilt through the normal Kustomize build, picking up filesystem changes.

## Filesystem support

The public filesystem helpers are in `pkg/util/fs`: `NewFsOnDisk`,
`NewMemoryFs`, `NewReadOnlyFs`, `NewFromIOFS`, and `NewBasePathFs`. Union
overlays are in `pkg/util/fs/union` and are created with `union.NewFs` plus
`union.WithOverride`, `union.WithOverrides`, or `union.WithOverlayFs`.

See [`../AGENTS.md`](../AGENTS.md), [`development.md`](development.md), and
[`fs-adapter.md`](fs-adapter.md).
