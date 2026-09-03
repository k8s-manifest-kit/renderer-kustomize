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

## Filesystem support

The public filesystem helpers are in `pkg/util/fs`: `NewFsOnDisk`,
`NewMemoryFs`, `NewReadOnlyFs`, `NewFromIOFS`, and `NewBasePathFs`. Union
overlays are in `pkg/util/fs/union` and are created with `union.NewFs` plus
`union.WithOverride`, `union.WithOverrides`, or `union.WithOverlayFs`.

See [`../AGENTS.md`](../AGENTS.md), [`development.md`](development.md), and
[`fs-adapter.md`](fs-adapter.md).
