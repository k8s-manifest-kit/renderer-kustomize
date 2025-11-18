# Kustomize Renderer Design

## Overview

The Kustomize renderer provides native integration with Kustomize, enabling programmatic rendering of Kustomize overlays within the k8s-manifest-kit ecosystem.

## Architecture

### Core Components

1. **Renderer** (`pkg/kustomize.go`)
   - Main entry point implementing `types.Renderer`
   - Manages kustomization loading and rendering
   - Handles caching at the kustomization-source level
   - Thread-safe for concurrent operations

2. **Source** (`pkg/kustomize.go`)
   - Defines kustomization path and configuration
   - Provides dynamic value functions for ConfigMap generation
   - Specifies load restrictions per source

3. **Options** (`pkg/kustomize_option.go`)
   - Functional options pattern for renderer configuration
   - Supports filters, transformers, caching, load restrictions

4. **Engine** (`pkg/kustomize_engine.go`)
   - Wraps Kustomize SDK for kustomization processing
   - Handles filesystem abstractions
   - Manages plugin configuration

5. **Filesystem Adapters** (`pkg/fs/`)
   - Afero-based implementation of `filesys.FileSystem`
   - Supports embedded, in-memory, and union filesystems
   - Organized by type: `pkg/fs/union/` for union filesystems
   - Flexible storage backends via functional options
   - See [Filesystem Adapters Guide](fs-adapter.md)

6. **Engine Convenience** (`pkg/engine.go`)
   - `NewEngine()` function for simple single-kustomization scenarios
   - Wraps renderer creation with engine setup

## Key Design Decisions

### 1. Filesystem Abstraction

The renderer uses Kustomize's `filesys.FileSystem` interface, enabling:
- Local filesystem access via `filesys.MakeFsOnDisk()` or `fs.NewFsOnDisk()`
- In-memory filesystems via `fs.NewMemoryFs()`
- Embedded filesystems via `fs.NewFromIOFS()` (e.g., embed.FS)
- Union filesystems via `fs.NewUnionFs()` for dynamic value injection
- Testing with mock filesystems

See [Filesystem Adapters](fs-adapter.md) for detailed usage guide.

### 2. Dynamic Values via ConfigMap

Values are injected as a Kustomize ConfigMap:

```go
Source{
    Path: "/path/to/kustomization",
    Values: func(ctx context.Context) (map[string]string, error) {
        return map[string]string{"replicas": "3"}, nil
    },
}
```

The renderer dynamically creates a `values.yaml` ConfigMap and patches the kustomization to include it.

### 3. Load Restrictions

Kustomize load restrictions control what files can be accessed:
- **LoadRestrictionsRootOnly** (default): Only files within kustomization root
- **LoadRestrictionsNone**: No restrictions

Can be set per-renderer or per-source:

```go
kustomize.WithLoadRestrictions(kustomizetypes.LoadRestrictionsNone)
```

### 4. Caching Strategy

Caching uses the same pattern as other renderers:
- Cache key: kustomization path + values hash
- TTL-based expiration
- Deep cloning for cached results
- Transparent to caller

### 5. Source Annotations

When enabled, the renderer annotates resources with origin metadata:
- `k8s-manifest-kit.io/renderer`: `"kustomize"`
- `k8s-manifest-kit.io/source.path`: Kustomization path
- `k8s-manifest-kit.io/source.file`: Relative file path within kustomization

### 6. Thread Safety

The renderer is designed for concurrent use:
- Immutable configuration after creation
- Per-operation filesystem instances
- Cache with built-in concurrency support

## Error Handling

The renderer follows Go error wrapping conventions:
- Validation errors at creation time
- Kustomize SDK errors wrapped with context
- Clear error messages for common issues

## Testing Strategy

1. **Unit Tests**: Individual function validation
2. **Integration Tests**: Full rendering pipelines
3. **Cache Tests**: Verify caching behavior
4. **Annotation Tests**: Verify source tracking
5. **Load Restriction Tests**: Verify security boundaries

Test fixtures in `config/test/kustomizations/` provide realistic kustomizations.

## Performance Considerations

1. **Filesystem I/O**: Disk-based kustomizations incur filesystem overhead
2. **Kustomize Processing**: Complex overlays can be CPU-intensive
3. **Caching**: Essential for repeated renders of the same kustomization
4. **UnionFS Overhead**: In-memory layer adds minimal overhead

## Future Enhancements

Potential areas for expansion:
1. Kustomize plugin support
2. Remote base references (e.g., GitHub)
3. Helm chart transformer integration
4. Advanced patching strategies

## Recent Changes

- **v0.x.x**: Migrated from `pkg/unionfs/` to `pkg/fs/union/` with functional options pattern
- Refactored filesystem adapters into subpackages by type
- Added support for embedded filesystems via `io.FS` integration

## Related Documentation

- [Filesystem Adapters Guide](fs-adapter.md) - Comprehensive guide to filesystem implementations
- [Development Guide](development.md) - Development workflow and guidelines

