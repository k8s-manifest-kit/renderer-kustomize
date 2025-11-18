package union

import (
	"errors"
	"fmt"
	"maps"

	"github.com/spf13/afero"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/k8s-manifest-kit/renderer-kustomize/pkg/fs"
	"github.com/k8s-manifest-kit/renderer-kustomize/pkg/fs/adapter"
)

// Option is a functional option for configuring a union filesystem.
type Option func(*config) error

type config struct {
	overrides map[string][]byte
	overlay   filesys.FileSystem
}

// WithOverride adds a virtual file to the overlay layer.
// The file will be created in memory and shadow any file at the same path in the base filesystem.
func WithOverride(path string, content []byte) Option {
	return func(cfg *config) error {
		if cfg.overrides == nil {
			cfg.overrides = make(map[string][]byte)
		}
		cfg.overrides[path] = content

		return nil
	}
}

// WithOverrides adds multiple virtual files to the overlay layer.
// Files will be created in memory and shadow files at the same paths in the base filesystem.
func WithOverrides(overrides map[string][]byte) Option {
	return func(cfg *config) error {
		if cfg.overrides == nil {
			cfg.overrides = make(map[string][]byte)
		}

		maps.Copy(cfg.overrides, overrides)

		return nil
	}
}

// WithOverlayFs specifies a custom overlay filesystem instead of using an in-memory one.
// If provided, this takes precedence over individual file overrides.
func WithOverlayFs(overlay filesys.FileSystem) Option {
	return func(cfg *config) error {
		cfg.overlay = overlay

		return nil
	}
}

// NewFs creates a union filesystem that layers an overlay over a base filesystem.
// Writes go to the overlay, reads check the overlay first then fall back to the base.
// This uses Afero's CopyOnWriteFs for better union filesystem behavior.
//
// The base filesystem is typically read-only or represents the "source" files.
// Options can be used to specify file overrides or a custom overlay filesystem.
//
// Example with file overrides:
//
//	unionFs, err := union.NewFs(base,
//	    union.WithOverride("/path/file.yaml", []byte("content")),
//	    union.WithOverrides(map[string][]byte{
//	        "/another.yaml": []byte("more content"),
//	    }),
//	)
//
// Example with custom overlay:
//
//	overlay := fs.NewMemoryFs()
//	overlay.WriteFile("/file.txt", []byte("content"))
//	unionFs, err := union.NewFs(base, union.WithOverlayFs(overlay))
func NewFs(base filesys.FileSystem, opts ...Option) (filesys.FileSystem, error) {
	cfg := &config{
		overrides: make(map[string][]byte),
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	// Determine overlay filesystem
	overlay := cfg.overlay
	if overlay == nil {
		// Create an in-memory overlay filesystem
		overlay = fs.NewMemoryFs()

		// Write all overrides to the overlay
		for path, content := range cfg.overrides {
			if err := overlay.WriteFile(path, content); err != nil {
				return nil, fmt.Errorf("failed to write override %s: %w", path, err)
			}
		}
	}

	baseAdapter, ok := base.(interface{ Unwrap() afero.Fs })
	if !ok {
		return nil, errors.New("base filesystem must be created with fs package functions") //nolint:err113
	}

	overlayAdapter, ok := overlay.(interface{ Unwrap() afero.Fs })
	if !ok {
		return nil, errors.New("overlay filesystem must be created with fs package functions") //nolint:err113
	}

	// Use Afero's CopyOnWriteFs to create a union filesystem
	// CopyOnWriteFs writes go to the overlay, reads check overlay first then base
	unionFs := afero.NewCopyOnWriteFs(baseAdapter.Unwrap(), overlayAdapter.Unwrap())

	return adapter.New(unionFs), nil
}
