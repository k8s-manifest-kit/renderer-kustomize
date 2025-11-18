# Filesystem Adapters for Kustomize

This package provides an Afero-based filesystem adapter that implements kustomize's `filesys.FileSystem` interface, enabling flexible filesystem implementations for kustomize operations.

## Features

- **OS Filesystem**: Standard filesystem operations backed by the OS
- **In-Memory Filesystem**: Fast, temporary filesystem for testing
- **Embedded Filesystem**: Load kustomizations from Go embed.FS
- **Union Filesystem**: Layer modifications over base filesystems
- **Read-Only Wrapper**: Prevent modifications to existing filesystems
- **Base Path Restriction**: Sandbox operations to specific directories

## Quick Start

### Basic Filesystems

```go
// OS filesystem
fs := fs.NewFsOnDisk()

// In-memory filesystem (for testing)
memFs := fs.NewMemoryFs()
memFs.WriteFile("/test.yaml", []byte("content"))
```

### Embedded Filesystems

Load kustomizations from files embedded in your binary:

```go
//go:embed kustomizations/*
var embeddedFS embed.FS

fsys, err := fs.NewFromIOFS(embeddedFS, "kustomizations")
if err != nil {
    log.Fatal(err)
}

renderer, err := kustomize.New(
    []kustomize.Source{{Path: "/my-app"}},
    kustomize.WithFileSystem(fsys),
)
```

### Union Filesystems

Layer modifications over a base filesystem using functional options:

```go
base := fs.NewFsOnDisk()

// Option 1: Individual overrides
union, err := fs.NewUnionFs(base,
    fs.WithOverride("/kustomization.yaml", modifiedKustomization),
    fs.WithOverride("/values.yaml", valuesContent),
)

// Option 2: Map of overrides
overrides := map[string][]byte{
    "/file1.yaml": []byte("content1"),
    "/file2.yaml": []byte("content2"),
}
union, err := fs.NewUnionFs(base, fs.WithOverrides(overrides))

// Option 3: Custom overlay filesystem
overlay := fs.NewMemoryFs()
overlay.WriteFile("/custom.yaml", []byte("content"))
union, err := fs.NewUnionFs(base, fs.WithOverlayFs(overlay))
```

## Use Cases

### Testing

Use in-memory filesystems for fast, isolated tests:

```go
func TestKustomization(t *testing.T) {
    memFs := fs.NewMemoryFs()
    memFs.WriteFile("/app/kustomization.yaml", kustomizationContent)
    memFs.WriteFile("/app/deployment.yaml", deploymentContent)
    
    renderer, _ := kustomize.New(
        []kustomize.Source{{Path: "/app"}},
        kustomize.WithFileSystem(memFs),
    )
    
    objects, err := renderer.Process(context.Background(), nil)
    // ... assertions ...
}
```

### Dynamic Value Injection

Inject dynamic values without modifying source files:

```go
base := fs.NewFsOnDisk()

values := map[string][]byte{
    "/config/values.yaml": []byte(fmt.Sprintf("replicas: %d", replicas)),
}

union, _ := fs.NewUnionFs(base, fs.WithOverrides(values))

renderer, _ := kustomize.New(
    []kustomize.Source{{Path: "./config"}},
    kustomize.WithFileSystem(union),
)
```

### Distribution with Embedded Files

Ship kustomizations as part of your binary:

```go
//go:embed templates/*
var templates embed.FS

func NewRenderer() (*kustomize.Renderer, error) {
    fsys, err := fs.NewFromIOFS(templates, "templates")
    if err != nil {
        return nil, err
    }
    
    return kustomize.New(
        []kustomize.Source{{Path: "/base"}},
        kustomize.WithFileSystem(fsys),
    )
}
```

## Migrating from pkg/unionfs

If you're using the legacy `pkg/unionfs` package, migration is straightforward.

**Note:** The internal kustomize renderer has already been migrated to use `pkg/fs/union`.
This section is for external users who may have been using the old API.

### Builder Pattern → Functional Options

**Before (deprecated):**

```go
import "github.com/k8s-manifest-kit/renderer-kustomize/pkg/unionfs"

builder := unionfs.NewBuilder(baseFs)
builder.WithOverride("/file1.yaml", content1)
builder.WithOverride("/file2.yaml", content2)
fs, err := builder.Build()
```

**After (recommended):**

```go
import (
    "github.com/k8s-manifest-kit/renderer-kustomize/pkg/fs"
    "github.com/k8s-manifest-kit/renderer-kustomize/pkg/fs/union"
)

unionFs, err := union.NewFs(baseFs,
    union.WithOverride("/file1.yaml", content1),
    union.WithOverride("/file2.yaml", content2),
)
```

### Using Maps

**Before:**

```go
overrides := map[string][]byte{
    "/file1.yaml": []byte("content1"),
    "/file2.yaml": []byte("content2"),
}
builder := unionfs.NewBuilder(baseFs)
builder.WithOverrides(overrides)
fs, err := builder.Build()
```

**After:**

```go
overrides := map[string][]byte{
    "/file1.yaml": []byte("content1"),
    "/file2.yaml": []byte("content2"),
}
unionFs, err := union.NewFs(baseFs, union.WithOverrides(overrides))
```

### Key Differences

1. **Package structure**: Union functionality moved to `pkg/fs/union` subpackage
2. **API style**: Builder pattern replaced with functional options
3. **Base filesystem**: Must now use `fs.NewFsOnDisk()` or other `fs` package constructors
4. **Implementation**: Uses Afero's CopyOnWriteFs for better reliability

## API Reference

### Constructors

- `NewFsOnDisk()` - OS-backed filesystem
- `NewMemoryFs()` - In-memory filesystem
- `NewFromIOFS(fs.FS, root)` - From io.FS (e.g., embed.FS)
- `NewReadOnlyFs(base)` - Read-only wrapper
- `NewBasePathFs(base, path)` - Restrict to base path
- `NewAferoAdapter(afero.Fs)` - Wrap custom Afero filesystem

### Union Filesystem Options

- `WithOverride(path, content)` - Add single file override
- `WithOverrides(map[string][]byte)` - Add multiple file overrides
- `WithOverlayFs(filesys.FileSystem)` - Use custom overlay filesystem

## Architecture

The package uses [Afero](https://github.com/spf13/afero) as the underlying filesystem abstraction, providing:

- **Portability**: Works across different filesystem implementations
- **Testing**: Easy to mock and test filesystem operations
- **Flexibility**: Support for various storage backends
- **Performance**: Optimized in-memory operations

The adapter implements all 13 methods of `filesys.FileSystem`:
- `Create`, `Open`, `ReadFile`, `WriteFile`
- `Mkdir`, `MkdirAll`, `RemoveAll`
- `ReadDir`, `Glob`, `Walk`
- `Exists`, `IsDir`, `CleanedAbs`

## Migration Path

This package is designed to eventually replace `pkg/unionfs`. Current status:

- ✅ Phase 1: New `pkg/fs/` package with Afero adapter (current)
- 🔄 Phase 2: Migrate internal usage to `pkg/fs/`
- 📅 Phase 3: Deprecate and remove `pkg/unionfs/`

## Examples

See `example_test.go` for comprehensive examples covering:
- Embedded filesystems
- In-memory filesystems
- Union filesystems with various options
- Read-only filesystems
- Dynamic value injection

## License

Same as parent project.

