//nolint:wrapcheck,mnd
package adapter

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Adapter wraps an afero.Fs to implement filesys.FileSystem.
type Adapter struct {
	fs afero.Fs
}

// New creates a filesys.FileSystem backed by the given afero.Fs.
func New(afs afero.Fs) filesys.FileSystem {
	return &Adapter{fs: afs}
}

// Create creates a file at the specified path.
func (a *Adapter) Create(path string) (filesys.File, error) {
	return a.fs.Create(path)
}

// Mkdir creates a directory.
func (a *Adapter) Mkdir(path string) error {
	return a.fs.Mkdir(path, 0777|os.ModeDir)
}

// MkdirAll creates a directory and all parent directories.
func (a *Adapter) MkdirAll(path string) error {
	return a.fs.MkdirAll(path, 0777|os.ModeDir)
}

// RemoveAll removes a path and all children.
func (a *Adapter) RemoveAll(path string) error {
	return a.fs.RemoveAll(path)
}

// Open opens a file for reading.
func (a *Adapter) Open(path string) (filesys.File, error) {
	return a.fs.Open(path)
}

// Exists checks if a path exists.
func (a *Adapter) Exists(path string) bool {
	_, err := a.fs.Stat(path)

	return err == nil
}

// IsDir checks if a path is a directory.
func (a *Adapter) IsDir(path string) bool {
	info, err := a.fs.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// ReadDir reads the directory entries.
func (a *Adapter) ReadDir(path string) ([]string, error) {
	entries, err := afero.ReadDir(a.fs, path)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(entries))
	for i, entry := range entries {
		result[i] = entry.Name()
	}

	return result, nil
}

// ReadFile reads the file contents.
func (a *Adapter) ReadFile(path string) ([]byte, error) {
	return afero.ReadFile(a.fs, path)
}

// WriteFile writes data to a file.
func (a *Adapter) WriteFile(path string, data []byte) error {
	return afero.WriteFile(a.fs, path, data, 0666)
}

// Glob returns paths matching the pattern.
func (a *Adapter) Glob(pattern string) ([]string, error) {
	return afero.Glob(a.fs, pattern)
}

// Walk walks the filesystem tree.
func (a *Adapter) Walk(path string, walkFn filepath.WalkFunc) error {
	return afero.Walk(a.fs, path, walkFn)
}

// CleanedAbs converts the given path into a directory and a file name.
// If the entire path is a directory, the file component is an empty string.
// The directory is represented as a ConfirmedDir.
func (a *Adapter) CleanedAbs(path string) (filesys.ConfirmedDir, string, error) {
	if path == "" {
		path = "."
	}

	// Try to make path absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("abs path error on %q: %w", path, err)
	}

	// For OsFs, resolve symlinks. For other filesystems, use as-is.
	// Check if the underlying fs supports EvalSymlinks
	resolvedPath := absPath
	if _, ok := a.fs.(*afero.OsFs); ok {
		deLinked, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", "", fmt.Errorf("evalsymlink failure on %q: %w", path, err)
		}
		resolvedPath = deLinked
	}

	// Check if path exists
	info, err := a.fs.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist}
		}

		return "", "", fmt.Errorf("stat error on %q: %w", path, err)
	}

	// If it's a directory, return the directory with empty filename
	if info.IsDir() {
		return filesys.ConfirmedDir(resolvedPath), "", nil
	}

	// It's a file, split into directory and filename
	dir := filepath.Dir(resolvedPath)
	file := filepath.Base(resolvedPath)

	return filesys.ConfirmedDir(dir), file, nil
}

// File adapts an afero.File to implement filesys.File if needed.
// In most cases, afero.File already implements the necessary interface.
var _ filesys.File = (afero.File)(nil)

// Unwrap returns the underlying afero.Fs for advanced usage.
// This is useful when you need to access Afero-specific functionality
// or pass the filesystem to other Afero-based tools.
func (a *Adapter) Unwrap() afero.Fs {
	return a.fs
}

// Ensure Adapter implements filesys.FileSystem.
var _ filesys.FileSystem = (*Adapter)(nil)
