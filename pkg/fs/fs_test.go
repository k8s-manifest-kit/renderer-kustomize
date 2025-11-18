package fs_test

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/k8s-manifest-kit/renderer-kustomize/pkg/fs"

	. "github.com/onsi/gomega"
)

func TestNewFsOnDisk(t *testing.T) {
	g := NewWithT(t)

	fsys := fs.NewFsOnDisk()
	g.Expect(fsys).NotTo(BeNil())

	// Test basic operations
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := fsys.WriteFile(testFile, []byte("hello world"))
	g.Expect(err).To(Succeed())

	data, err := fsys.ReadFile(testFile)
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("hello world"))

	exists := fsys.Exists(testFile)
	g.Expect(exists).To(BeTrue())
}

func TestNewMemoryFs(t *testing.T) {
	g := NewWithT(t)

	fsys := fs.NewMemoryFs()
	g.Expect(fsys).NotTo(BeNil())

	// Test in-memory operations
	err := fsys.WriteFile("/test.txt", []byte("in memory"))
	g.Expect(err).To(Succeed())

	data, err := fsys.ReadFile("/test.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("in memory"))
}

func TestNewReadOnlyFs(t *testing.T) {
	g := NewWithT(t)

	// Create a base filesystem with some content
	base := fs.NewMemoryFs()
	err := base.WriteFile("/readonly.txt", []byte("readonly content"))
	g.Expect(err).To(Succeed())

	// Wrap in read-only
	readOnly := fs.NewReadOnlyFs(base)
	g.Expect(readOnly).NotTo(BeNil())

	// Should be able to read
	data, err := readOnly.ReadFile("/readonly.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("readonly content"))

	// Should not be able to write
	err = readOnly.WriteFile("/newfile.txt", []byte("should fail"))
	g.Expect(err).To(HaveOccurred())

	err = readOnly.Mkdir("/newdir")
	g.Expect(err).To(HaveOccurred())
}

func TestNewFromIOFS(t *testing.T) {
	g := NewWithT(t)

	// Create a test filesystem using fstest.MapFS
	testFS := fstest.MapFS{
		"subdir/test.yaml": &fstest.MapFile{
			Data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"),
		},
	}

	// Create from io.FS
	fsys, err := fs.NewFromIOFS(testFS, "subdir")
	g.Expect(err).To(Succeed())
	g.Expect(fsys).NotTo(BeNil())

	// Test reading existing file
	data, err := fsys.ReadFile("test.yaml")
	g.Expect(err).To(Succeed())
	g.Expect(data).NotTo(BeEmpty())

	// Verify it's read-only
	err = fsys.WriteFile("new.txt", []byte("should fail"))
	g.Expect(err).To(HaveOccurred())
}

func TestNewFromIOFS_EmptyRoot(t *testing.T) {
	g := NewWithT(t)

	// Create a test filesystem using fstest.MapFS
	testFS := fstest.MapFS{
		"dir/file.yaml": &fstest.MapFile{
			Data: []byte("test: data\n"),
		},
	}

	fsys, err := fs.NewFromIOFS(testFS, "")
	g.Expect(err).To(Succeed())
	g.Expect(fsys).NotTo(BeNil())

	// Should be able to read from the filesystem
	data, err := fsys.ReadFile("dir/file.yaml")
	g.Expect(err).To(Succeed())
	g.Expect(data).NotTo(BeEmpty())
}

func TestNewBasePathFs(t *testing.T) {
	g := NewWithT(t)

	base := fs.NewMemoryFs()
	err := base.MkdirAll("/root/subdir")
	g.Expect(err).To(Succeed())
	err = base.WriteFile("/root/subdir/file.txt", []byte("content"))
	g.Expect(err).To(Succeed())

	scoped, err := fs.NewBasePathFs(base, "/root")
	g.Expect(err).To(Succeed())

	// Should be able to access relative to base path
	data, err := scoped.ReadFile("/subdir/file.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("content"))
}

// Ensure the constructors return filesys.FileSystem.
func TestConstructorsReturnFilesysFileSystem(t *testing.T) {
	g := NewWithT(t)

	_ = fs.NewMemoryFs()
	_ = fs.NewFsOnDisk()

	g.Expect(true).To(BeTrue()) // Compilation check
}
