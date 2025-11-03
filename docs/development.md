# Kustomize Renderer Development Guide

## Setup

### Prerequisites

- Go 1.24 or later
- Make
- golangci-lint

### Getting Started

```bash
# Clone and navigate
cd /path/to/k8s-manifest-kit/renderer-kustomize

# Install dependencies
go mod download

# Run tests
make test

# Run linter
make lint
```

## Project Structure

```
renderer-kustomize/
├── pkg/
│   ├── kustomize.go          # Main renderer implementation
│   ├── kustomize_option.go   # Functional options
│   ├── kustomize_support.go  # Helper functions
│   ├── kustomize_engine.go   # Kustomize SDK wrapper
│   ├── kustomize_test.go     # Tests
│   ├── engine.go             # NewEngine convenience
│   ├── engine_test.go        # NewEngine tests
│   └── unionfs/
│       ├── unionfs.go        # Union filesystem
│       └── unionfs_test.go   # UnionFS tests
├── config/test/
│   └── kustomizations/       # Test fixtures
├── docs/
│   ├── design.md            # Architecture documentation
│   └── development.md       # This file
├── .golangci.yml            # Linter configuration
├── Makefile                 # Build automation
├── go.mod                   # Go module definition
└── README.md                # Project overview
```

## Coding Conventions

### Go Style

Follow standard Go conventions plus:
- Each function parameter has its own type declaration
- Use multiline formatting for functions with 3+ parameters
- Prefer explicit types when they aid readability

### Error Handling

- Return errors as the last parameter
- Use `fmt.Errorf` with `%w` verb for wrapping
- Handle errors at appropriate abstraction level

### Testing with Gomega

Use vanilla Gomega assertions:

```go
import . "github.com/onsi/gomega"

func TestExample(t *testing.T) {
    g := NewWithT(t)
    result, err := someFunction()
    
    g.Expect(err).ShouldNot(HaveOccurred())
    g.Expect(result).Should(HaveLen(3))
}
```

### Documentation

- Comments explain *why*, not *what*
- Focus on non-obvious behavior and edge cases
- Skip boilerplate docstrings unless they add value

## Development Workflow

### Making Changes

1. **Write Tests First**: Add test cases for new functionality
2. **Implement**: Make minimal changes to fulfill requirements
3. **Run Tests**: `make test`
4. **Run Linter**: `make lint`
5. **Fix Issues**: Address any linter warnings

### Adding New Features

When adding features to the renderer:

1. **Consider the Interface**: Will this change `types.Renderer`?
2. **Options Pattern**: Use functional options for configuration
3. **Thread Safety**: Ensure concurrent use is safe
4. **Error Messages**: Provide clear, actionable error messages
5. **Documentation**: Update docs/ and CLAUDE.md

### Testing Guidelines

**Unit Tests** (`*_test.go`):
- Test individual functions in isolation
- Use test fixtures from `config/test/`
- Mock external dependencies when appropriate

**Integration Tests**:
- Test full rendering pipelines
- Verify caching behavior
- Test source annotations
- Validate load restrictions

**Test Fixtures**:
- Keep kustomizations simple and focused
- Document what each fixture tests
- Use realistic Kubernetes resources

## Common Tasks

### Adding a New Option

1. Add field to `RendererOptions` in `kustomize_option.go`
2. Create `WithXxx()` function
3. Add test in `kustomize_option_test.go`
4. Update `design.md` documentation

### Debugging Tests

```bash
# Run specific test
go test -v ./pkg -run TestName

# Run with race detector
go test -race ./pkg/...

# Show coverage
go test -cover ./pkg/...
```

### Working with Kustomize SDK

The renderer uses `sigs.k8s.io/kustomize/api` and `sigs.k8s.io/kustomize/kyaml`:

```go
import (
    "sigs.k8s.io/kustomize/api/krusty"
    "sigs.k8s.io/kustomize/api/resmap"
    "sigs.k8s.io/kustomize/kyaml/filesys"
)
```

Key concepts:
- **Kustomizer**: Main processor for kustomizations
- **ResMap**: Resource map (kustomization output)
- **FileSystem**: Abstraction for file access

## Troubleshooting

### Common Issues

**Import paths not resolving**:
```bash
go mod tidy
go get github.com/k8s-manifest-kit/pkg@latest
```

**Linter errors**:
```bash
make lint
# Fix reported issues, then:
go fmt ./...
```

**Test failures**:
- Check test fixture paths are correct
- Verify kustomizations are valid (`kustomize build`)
- Look for filesystem permission issues

## CI/CD

The repository uses GitHub Actions for CI:
- **Lint**: golangci-lint with aggressive rules
- **Test**: Go test with race detector
- **Build**: Verify compilation

Ensure all checks pass before merging.

## Contributing

When contributing:
1. Follow the coding conventions
2. Add tests for new functionality
3. Update documentation
4. Ensure CI passes
5. Keep changes focused and minimal

