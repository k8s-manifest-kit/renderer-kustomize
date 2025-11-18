# AI Assistant Guide for k8s-manifest-kit/renderer-kustomize

## Quick Reference

This is the **Kustomize renderer** for the k8s-manifest-kit ecosystem. It provides programmatic rendering of Kustomize overlays with features like caching, filtering, transformation, and source tracking.

### Repository Structure
- `pkg/` - Main renderer implementation and filesystem adapters
- `pkg/util/fs/` - Afero-based filesystem adapter with subpackages by type
- `pkg/util/fs/union/` - Union filesystem implementation
- `pkg/util/io/` - I/O utility functions (stderr suppression, etc.)
- `config/test/kustomizations/` - Test fixtures
- `docs/` - Architecture and development documentation

### Key Files
- `pkg/kustomize.go` - Main renderer (`New()`, `Process()`)
- `pkg/kustomize_option.go` - Functional options (`WithCache()`, `WithFileSystem()`, etc.)
- `pkg/kustomize_engine.go` - Kustomize SDK wrapper
- `pkg/engine.go` - Convenience function (`NewEngine()`)
- `pkg/util/fs/` - Filesystem adapters for flexible storage backends
- `pkg/util/fs/union/` - Union filesystem for dynamic value injection
- `pkg/util/io/` - I/O utility functions

### Related Repositories
- `github.com/k8s-manifest-kit/engine` - Core engine and types
- `github.com/k8s-manifest-kit/pkg` - Shared utilities (cache, errors, etc.)

## Common Tasks

### Understanding the Code

**Q: How does the renderer work?**
1. Sources specify kustomization paths and optional dynamic values
2. Values are injected as ConfigMaps via unionfs overlay
3. Kustomize SDK processes the kustomization
4. Results are filtered/transformed per pipeline configuration
5. Resources are cached based on path + values hash

**Q: What's the difference between `New()` and `NewEngine()`?**
- `New()` creates a `Renderer` implementing `types.Renderer`
- `NewEngine()` creates an `engine.Engine` with a single Kustomize renderer (convenience)

**Q: How do dynamic values work?**
Values are provided via `Source.Values` function, which returns `map[string]string`. The renderer:
1. Creates a ConfigMap named `values` with these entries
2. Uses unionfs to overlay `values.yaml` in the kustomization directory
3. Patches the kustomization to include this ConfigMap

### Making Changes

**Adding a renderer option:**
1. Add field to `RendererOptions` struct
2. Create `WithXxx()` function returning `RendererOption`
3. Add test coverage
4. Update documentation

**Adding a source option:**
1. Add field to `Source` struct
2. Update `Validate()` if needed
3. Handle in renderer processing logic
4. Add test coverage

**Modifying caching:**
- Cache logic is in `pkg/kustomize.go` `Process()`
- Cache key: kustomization path + values hash
- Uses `github.com/k8s-manifest-kit/pkg/util/cache`

### Testing

**Run tests:**
```bash
make test
```

**Test structure:**
- Unit tests in `pkg/*_test.go`
- Test fixtures in `config/test/kustomizations/`
- Uses Gomega assertions (dot import)

**Key test files:**
- `kustomize_test.go` - Main renderer tests
- `engine_test.go` - NewEngine tests
- `unionfs/unionfs_test.go` - UnionFS tests

### Debugging

**Common issues:**
1. **Load restrictions**: Default is `LoadRestrictionsRootOnly`
2. **Filesystem paths**: Must be absolute or relative to working directory
3. **Values injection**: Requires kustomization to reference the ConfigMap
4. **Import paths**: Must use `github.com/k8s-manifest-kit/*` (not old `lburgazzoli/*`)

**Useful debugging:**
```bash
# Run specific test
go test -v ./pkg -run TestRendererBasic

# Check Kustomize directly
kustomize build config/test/kustomizations/simple
```

## Architecture Notes

### Thread Safety
The renderer is thread-safe:
- Configuration is immutable after creation
- Per-operation filesystem instances
- Cache has built-in concurrency support

### Filesystem Adapters
Afero-based filesystem adapters (`pkg/util/fs/`):
- Supports embedded filesystems (embed.FS)
- In-memory filesystems for testing
- Union filesystems with functional options (`pkg/util/fs/union/`)
- Read-only wrappers and base path restrictions
- Organized by filesystem type in subpackages
- See `docs/fs-adapter.md` for details

### Pipeline Integration
The renderer integrates with the three-level pipeline:
1. **Renderer-specific** (via `New()` options)
2. **Engine-level** (via `engine.New()` options)
3. **Render-time** (via `engine.Render()` options)

## Development Tips

1. **Follow established patterns** from helm/gotemplate renderers
2. **Use functional options** for all configuration
3. **Document non-obvious behavior** in comments
4. **Test with realistic kustomizations** in config/test/
5. **Check the linter** (`make lint`) - it's aggressive
6. **Keep imports organized** per `.golangci.yml` rules

## Code Review Checklist

When reviewing changes:
- [ ] Tests added for new functionality
- [ ] Error messages are clear and actionable
- [ ] Documentation updated (design.md, development.md)
- [ ] Follows Go conventions (parameter types, etc.)
- [ ] Thread safety considered
- [ ] Linter passes
- [ ] Imports use new k8s-manifest-kit paths

## Common Patterns

### Creating a renderer:
```go
r, err := kustomize.New(
    []kustomize.Source{{Path: "/path/to/kustomization"}},
    kustomize.WithCache(cache.WithTTL(5*time.Minute)),
)
```

### Using NewEngine:
```go
e, err := kustomize.NewEngine(
    kustomize.Source{Path: "/path/to/kustomization"},
    kustomize.WithLoadRestrictions(kustomizetypes.LoadRestrictionsNone),
)
```

### Dynamic values:
```go
kustomize.Source{
    Path: "/path/to/kustomization",
    Values: kustomize.Values(map[string]string{"replicas": "3"}),
}
```

## Questions?

Check:
1. `docs/design.md` - Architecture and design decisions
2. `docs/fs-adapter.md` - Filesystem adapters guide
3. `docs/development.md` - Development workflow
4. `pkg/*_test.go` - Usage examples
5. Parent repository documentation at github.com/k8s-manifest-kit

