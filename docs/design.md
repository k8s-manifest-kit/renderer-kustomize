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

5. **Filesystem Adapters** (`pkg/util/fs/`)
   - Afero-based implementation of `filesys.FileSystem`
   - Supports embedded, in-memory, and union filesystems
   - Organized by type: `pkg/util/fs/union/` for union filesystems
   - Flexible storage backends via functional options
   - See [Filesystem Adapters Guide](fs-adapter.md)

6. **Engine Convenience** (`pkg/engine.go`)
   - `NewEngine()` function for simple single-kustomization scenarios
   - Wraps renderer creation with engine setup

## Library Design Principles

This library follows specific design principles to remain a composable, unopinionated building block for applications.

### No Logging by Design

This library intentionally does **not** include any logging functionality. This is a deliberate architectural decision based on library design best practices:

- **Libraries should not impose logging frameworks** on consuming applications
- **Log output pollutes application logs** with library-specific formatting and levels
- **The consuming application should control all logging decisions**, including when, where, and how to log
- **Avoids dependency coupling** to specific logging libraries (logrus, zap, slog, etc.)

**Instead of logging, this library provides:**
- Rich error context through Go's error wrapping (`fmt.Errorf` with `%w`)
- Clear, descriptive error messages that chain context from lower layers
- Full stack traces through wrapped errors
- Applications can inspect errors and log at their discretion

### Observability at the Right Layer

Metrics and observability concerns are **intentionally delegated to appropriate layers**:

**Cache metrics belong in the cache layer:**
- The renderer accepts `cache.Interface[[]unstructured.Unstructured]` via dependency injection
- Metrics collection is the responsibility of the cache implementation, not the renderer
- Follows the **dependency inversion principle**: renderer depends on interface, not implementation
- Users can bring their own cache with built-in metrics, tracing, or monitoring

**Why this is correct:**
- **Single Responsibility**: Renderer renders, cache caches, metrics measure
- **No coupling**: Renderer doesn't depend on metric collection strategies
- **Flexibility**: Different cache implementations can provide different observability approaches
- **Testability**: Easy to test with simple in-memory cache or sophisticated monitoring cache

### Unopinionated Library Philosophy

This library is designed to be a **focused, composable building block**:

**What this means:**
- **Does one thing well**: Renders Kustomize manifests programmatically
- **No hidden side effects**: No file writes (except through explicit filesystem), no logging, no metrics
- **Cross-cutting concerns delegated**: Logging, metrics, tracing belong in the application layer
- **Clean interfaces**: Filesystem, cache, filters, transformers all injectable
- **Composable**: Works with any cache implementation, filesystem, or pipeline

**Benefits of this approach:**
- Library remains lightweight and focused
- No unnecessary dependencies
- Applications maintain full control over observability
- Easy to integrate into existing systems with their own observability infrastructure
- Library can be used in diverse contexts (CLI tools, web services, batch processors)

### What the Library DOES Provide

While avoiding opinions about cross-cutting concerns, the library provides everything needed for robust error handling and flexibility:

1. **Rich Error Context**
   ```go
   return fmt.Errorf("failed to run kustomize for path %q: %w", holder.Path, err)
   ```
   - Errors chain context from lower layers
   - Easy to inspect and handle at application level

2. **Clear Error Messages**
   - Descriptive messages explain what failed and why
   - Context includes paths, values, and operation details
   - Validation errors at creation time prevent runtime surprises

3. **Interface-Based Abstractions**
   - `filesys.FileSystem`: Bring your own filesystem (OS, memory, union, embedded)
   - `cache.Interface`: Bring your own cache with metrics/observability
   - `types.Filter` and `types.Transformer`: Inject custom processing

4. **Functional Options Pattern**
   - `WithCache()`, `WithFileSystem()`, `WithFilters()`, etc.
   - Flexible configuration without breaking API compatibility
   - Optional features remain optional

5. **No Global State**
   - All configuration via constructor and options
   - Thread-safe by design
   - Multiple independent renderer instances coexist

This design philosophy ensures the library remains a **professional, maintainable, and composable component** suitable for production systems.

## Key Design Decisions

### 1. Filesystem Abstraction

The renderer uses Kustomize's `filesys.FileSystem` interface, enabling:
- Local filesystem access via `filesys.MakeFsOnDisk()` or `utilfs.NewFsOnDisk()`
- In-memory filesystems via `utilfs.NewMemoryFs()`
- Embedded filesystems via `utilfs.NewFromIOFS()` (e.g., embed.FS)
- Union filesystems via `utilfs.NewUnionFs()` for dynamic value injection
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

- **v0.x.x**: Migrated from `pkg/unionfs/` to `pkg/util/fs/union/` with functional options pattern
- Refactored filesystem adapters into subpackages by type under `pkg/util/`
- Added support for embedded filesystems via `io.FS` integration
- Reorganized package structure: moved `pkg/fs/` to `pkg/util/fs/` for better grouping

## Related Documentation

- [Filesystem Adapters Guide](fs-adapter.md) - Comprehensive guide to filesystem implementations
- [Development Guide](development.md) - Development workflow and guidelines

