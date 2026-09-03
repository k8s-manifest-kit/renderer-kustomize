# Filesystem Adapters

The Kustomize renderer can work with the filesystem implementations in
`pkg/util/fs` or with an adapter around another filesystem library.

## Public filesystem constructors

```go
disk := fs.NewFsOnDisk()
memory := fs.NewMemoryFs()
readonly := fs.NewReadOnlyFs(memory)
fromIOFS, err := fs.NewFromIOFS(os.DirFS("manifests"), ".")
if err != nil {
    return err
}
scoped, err := fs.NewBasePathFs(fromIOFS, "overlays/dev")
if err != nil {
    return err
}
```

The exact constructor arguments are defined by the package's Go API; use the
implementation package docs and tests as the authoritative contract when
adding a new adapter.

## Union overlays

Use `pkg/util/fs/union` for a base filesystem with virtual replacements:

```go
overlay, err := union.NewFs(base,
    union.WithOverride("overlays/dev/kustomization.yaml", content),
)
if err != nil {
    return err
}
```

`union.WithOverrides` applies multiple file replacements and
`union.WithOverlayFs` layers another filesystem. Overlay reads take precedence
over the base filesystem while unrelated paths continue to resolve from the
base.

## Adapter guidance

Adapters must preserve filesystem path semantics, return standard filesystem
errors, and avoid allowing an overlay to escape its intended base path. Add
tests for `Open`, directory reads, missing paths, and overlay precedence.

See [`design.md`](design.md), [`development.md`](development.md), and
[`../AGENTS.md`](../AGENTS.md).
