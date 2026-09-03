# Kustomize Renderer

`renderer-kustomize` renders Kustomizations through the shared engine
interfaces. Sources can use an on-disk, in-memory, or adapter-backed
filesystem and can configure Kustomize load restrictions and warning handling.

## Installation

```bash
go get github.com/k8s-manifest-kit/renderer-kustomize
```

## Quick start

```go
e, err := kustomize.NewEngine(kustomize.Source{
    Path: "./overlays/dev",
})
if err != nil {
    return err
}

objects, err := e.Render(ctx)
```

`Source.Values` exposes a virtual `Path/values.yaml` file. It is not applied
automatically: the Kustomization must consume that file through its own
replacement, generator, or patch configuration. The default load restriction
is `LoadRestrictionsRootOnly`.

See [`docs/design.md`](docs/design.md), [`docs/development.md`](docs/development.md),
[`docs/fs-adapter.md`](docs/fs-adapter.md), and [`AGENTS.md`](AGENTS.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).
