package fs

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/spf13/afero"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/k8s-manifest-kit/renderer-kustomize/pkg/util/fs/adapter"
)

// NewFsOnDisk creates a filesys.FileSystem backed by the OS filesystem.
// This is equivalent to filesys.MakeFsOnDisk() but using the Afero adapter.
func NewFsOnDisk() filesys.FileSystem {
	return adapter.New(afero.NewOsFs())
}

// NewMemoryFs creates an in-memory filesys.FileSystem.
// Useful for testing or when you need a temporary, non-persistent filesystem.
func NewMemoryFs() filesys.FileSystem {
	return adapter.New(afero.NewMemMapFs())
}

// NewReadOnlyFs creates a read-only wrapper around the given filesys.FileSystem.
// All write operations (Create, WriteFile, Mkdir, MkdirAll, RemoveAll) will return errors.
func NewReadOnlyFs(base filesys.FileSystem) filesys.FileSystem {
	// If base is an Afero adapter, we can wrap its underlying Fs
	if unwrapper, ok := base.(interface{ Unwrap() afero.Fs }); ok {
		return adapter.New(afero.NewReadOnlyFs(unwrapper.Unwrap()))
	}

	// Otherwise, create a new memory filesystem and copy files on demand
	// This is a simplified approach - for full read-only support of non-Afero filesystems,
	// you'd need a more sophisticated wrapper
	return &readOnlyWrapper{base: base}
}

// NewFromIOFS creates a filesys.FileSystem from an fs.FS (e.g., embed.FS).
// The root parameter specifies the root directory within the fs.FS to use as the base.
// If root is empty, the fs.FS root is used.
//
// Note: The resulting filesystem is read-only for the io.FS contents.
// Write operations will fail unless you layer it with a union filesystem.
func NewFromIOFS(fsys fs.FS, root string) (filesys.FileSystem, error) {
	// Use Afero's IOFS adapter to bridge io.FS to afero.Fs
	baseFs := afero.FromIOFS{FS: fsys}

	// If a root is specified, create a BasePathFs
	var resultFs afero.Fs = baseFs
	if root != "" {
		// Clean the root path
		root = filepath.Clean(root)
		resultFs = afero.NewBasePathFs(baseFs, root)
	}

	// Wrap in read-only since io.FS is inherently read-only
	readOnlyFs := afero.NewReadOnlyFs(resultFs)

	return adapter.New(readOnlyFs), nil
}

// NewBasePathFs creates a filesys.FileSystem that restricts operations to a base path.
// All file operations are performed relative to the given base path.
// This is useful for sandboxing operations to a specific directory.
func NewBasePathFs(base filesys.FileSystem, basePath string) (filesys.FileSystem, error) {
	// If base is an Afero adapter, wrap its underlying Fs
	if unwrapper, ok := base.(interface{ Unwrap() afero.Fs }); ok {
		return adapter.New(afero.NewBasePathFs(unwrapper.Unwrap(), basePath)), nil
	}

	return nil, errors.New("base filesystem must be created with fs package functions") //nolint:err113
}

// readOnlyWrapper provides a simple read-only wrapper for non-Afero filesystems.
type readOnlyWrapper struct {
	base filesys.FileSystem
}

func (r *readOnlyWrapper) Create(_ string) (filesys.File, error) {
	return nil, errors.New("create not supported on read-only filesystem") //nolint:err113
}

func (r *readOnlyWrapper) Mkdir(_ string) error {
	return errors.New("mkdir not supported on read-only filesystem") //nolint:err113
}

func (r *readOnlyWrapper) MkdirAll(_ string) error {
	return errors.New("mkdirall not supported on read-only filesystem") //nolint:err113
}

func (r *readOnlyWrapper) RemoveAll(_ string) error {
	return errors.New("removeall not supported on read-only filesystem") //nolint:err113
}

func (r *readOnlyWrapper) WriteFile(_ string, _ []byte) error {
	return errors.New("writefile not supported on read-only filesystem") //nolint:err113
}

func (r *readOnlyWrapper) Open(path string) (filesys.File, error) {
	return r.base.Open(path) //nolint:wrapcheck
}

func (r *readOnlyWrapper) Exists(path string) bool {
	return r.base.Exists(path)
}

func (r *readOnlyWrapper) IsDir(path string) bool {
	return r.base.IsDir(path)
}

func (r *readOnlyWrapper) ReadDir(path string) ([]string, error) {
	return r.base.ReadDir(path) //nolint:wrapcheck
}

func (r *readOnlyWrapper) ReadFile(path string) ([]byte, error) {
	return r.base.ReadFile(path) //nolint:wrapcheck
}

func (r *readOnlyWrapper) Glob(pattern string) ([]string, error) {
	return r.base.Glob(pattern) //nolint:wrapcheck
}

func (r *readOnlyWrapper) Walk(path string, walkFn filepath.WalkFunc) error {
	return r.base.Walk(path, walkFn) //nolint:wrapcheck
}

func (r *readOnlyWrapper) CleanedAbs(path string) (filesys.ConfirmedDir, string, error) {
	return r.base.CleanedAbs(path) //nolint:wrapcheck
}

var _ filesys.FileSystem = (*readOnlyWrapper)(nil)
