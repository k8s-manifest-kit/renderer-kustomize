//nolint:wrapcheck
package unionfs

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"

	"sigs.k8s.io/kustomize/kyaml/filesys"

	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	// ErrRemoveAllNotSupported is returned when RemoveAll is called on a union filesystem.
	ErrRemoveAllNotSupported = errors.New("RemoveAll not supported on union filesystem")
)

// unionFS provides a union filesystem that layers an in-memory FS over a delegate FS.
// Writes go to the memory layer, reads check memory first then fall back to delegate.
type unionFS struct {
	memory   filesys.FileSystem
	delegate filesys.FileSystem
}

func (u *unionFS) ReadFile(path string) ([]byte, error) {
	if u.memory.Exists(path) {
		return u.memory.ReadFile(path)
	}

	return u.delegate.ReadFile(path)
}

func (u *unionFS) WriteFile(path string, data []byte) error {
	return u.memory.WriteFile(path, data)
}

func (u *unionFS) Mkdir(path string) error {
	return u.memory.Mkdir(path)
}

func (u *unionFS) MkdirAll(path string) error {
	return u.memory.MkdirAll(path)
}

func (u *unionFS) RemoveAll(_ string) error {
	return ErrRemoveAllNotSupported
}

func (u *unionFS) Create(path string) (filesys.File, error) {
	return u.memory.Create(path)
}

func (u *unionFS) Open(path string) (filesys.File, error) {
	if u.memory.Exists(path) {
		return u.memory.Open(path)
	}

	return u.delegate.Open(path)
}

func (u *unionFS) Exists(path string) bool {
	return u.memory.Exists(path) || u.delegate.Exists(path)
}

func (u *unionFS) IsDir(path string) bool {
	if u.memory.Exists(path) {
		return u.memory.IsDir(path)
	}

	return u.delegate.IsDir(path)
}

func (u *unionFS) ReadDir(path string) ([]string, error) {
	res := sets.New[string]()

	// Get files from memory layer
	if u.memory.Exists(path) && u.memory.IsDir(path) {
		files, err := u.memory.ReadDir(path)
		if err != nil {
			return nil, err
		}

		res.Insert(files...)
	}

	// Get files from delegate layer (deduplicate)
	if u.delegate.Exists(path) && u.delegate.IsDir(path) {
		files, err := u.delegate.ReadDir(path)
		if err != nil {
			return nil, err
		}

		res.Insert(files...)
	}

	return res.UnsortedList(), nil
}

func (u *unionFS) Glob(pattern string) ([]string, error) {
	res := sets.New[string]()

	// Get matches from memory layer
	files, err := u.memory.Glob(pattern)
	if err != nil {
		return nil, err
	}

	res.Insert(files...)

	// Get matches from delegate layer (deduplicate)
	files, err = u.delegate.Glob(pattern)
	if err != nil {
		return nil, err
	}

	res.Insert(files...)

	return res.UnsortedList(), nil
}

func (u *unionFS) Walk(path string, walkFn filepath.WalkFunc) error {
	visited := make(map[string]bool)

	// Walk memory layer first
	if u.memory.Exists(path) {
		err := u.memory.Walk(path, func(p string, info fs.FileInfo, err error) error {
			visited[p] = true

			return walkFn(p, info, err)
		})
		if err != nil {
			return err
		}
	}

	// Walk delegate layer (skip visited)
	if u.delegate.Exists(path) {
		err := u.delegate.Walk(path, func(p string, info fs.FileInfo, err error) error {
			if visited[p] {
				return nil
			}

			return walkFn(p, info, err)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (u *unionFS) CleanedAbs(path string) (filesys.ConfirmedDir, string, error) {
	return u.delegate.CleanedAbs(path)
}

// Builder provides a fluent API for constructing a union filesystem.
type Builder struct {
	delegate  filesys.FileSystem
	overrides map[string][]byte
}

// NewBuilder creates a new union FS builder wrapping the given delegate filesystem.
func NewBuilder(delegate filesys.FileSystem) *Builder {
	return &Builder{
		delegate:  delegate,
		overrides: make(map[string][]byte),
	}
}

// WithOverride adds a virtual file to the memory layer.
func (b *Builder) WithOverride(path string, content []byte) *Builder {
	b.overrides[path] = content

	return b
}

// WithOverrides adds multiple virtual files to the memory layer.
func (b *Builder) WithOverrides(overrides map[string][]byte) *Builder {
	maps.Copy(b.overrides, overrides)

	return b
}

// Build creates the union filesystem with all configured overrides.
func (b *Builder) Build() (filesys.FileSystem, error) {
	memory := filesys.MakeFsInMemory()

	for path, content := range b.overrides {
		if err := memory.WriteFile(path, content); err != nil {
			return nil, fmt.Errorf("failed to write override %s: %w", path, err)
		}
	}

	return &unionFS{
		memory:   memory,
		delegate: b.delegate,
	}, nil
}
