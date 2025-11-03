package unionfs_test

import (
	"io/fs"
	"path/filepath"
	"testing"

	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/k8s-manifest-kit/renderer-kustomize/pkg/unionfs"

	. "github.com/onsi/gomega"
)

const (
	testFile1    = "file1.txt"
	testFile2    = "file2.txt"
	testContent1 = "content from memory"
	testContent2 = "content from delegate"
	testPattern  = "*.txt"
)

func TestUnionFS(t *testing.T) {

	t.Run("should read from memory layer first", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()
		err := delegate.WriteFile(testFile1, []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride(testFile1, []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		content, err := ufs.ReadFile(testFile1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content)).Should(Equal(testContent1))
	})

	t.Run("should fallback to delegate for reads", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()
		err := delegate.WriteFile(testFile1, []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).Build()
		g.Expect(err).ToNot(HaveOccurred())

		content, err := ufs.ReadFile(testFile1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content)).Should(Equal(testContent2))
	})

	t.Run("should write to memory layer only", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		ufs, err := unionfs.NewBuilder(delegate).Build()
		g.Expect(err).ToNot(HaveOccurred())

		err = ufs.WriteFile(testFile1, []byte(testContent1))
		g.Expect(err).ToNot(HaveOccurred())

		// File should exist in union FS
		g.Expect(ufs.Exists(testFile1)).Should(BeTrue())

		// File should NOT exist in delegate
		g.Expect(delegate.Exists(testFile1)).Should(BeFalse())

		// Read from union should get memory content
		content, err := ufs.ReadFile(testFile1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content)).Should(Equal(testContent1))
	})

	t.Run("should check existence in both layers", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()
		err := delegate.WriteFile(testFile1, []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride(testFile2, []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		// File in delegate
		g.Expect(ufs.Exists(testFile1)).Should(BeTrue())

		// File in memory
		g.Expect(ufs.Exists(testFile2)).Should(BeTrue())

		// Non-existent file
		g.Expect(ufs.Exists("nonexistent.txt")).Should(BeFalse())
	})

	t.Run("should merge directory listings", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()
		err := delegate.WriteFile("dir/file1.txt", []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())
		err = delegate.WriteFile("dir/file2.txt", []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride("dir/file2.txt", []byte(testContent1)).
			WithOverride("dir/file3.txt", []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		files, err := ufs.ReadDir("dir")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(files).Should(ConsistOf("file1.txt", "file2.txt", "file3.txt"))
	})

	t.Run("should merge glob results", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsOnDisk()
		tmpDir := t.TempDir()

		delegateFile1 := filepath.Join(tmpDir, "file1.txt")
		delegateFile2 := filepath.Join(tmpDir, "file2.txt")

		err := delegate.WriteFile(delegateFile1, []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())
		err = delegate.WriteFile(delegateFile2, []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride(delegateFile2, []byte(testContent1)).
			WithOverride(filepath.Join(tmpDir, "file3.txt"), []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		files, err := ufs.Glob(filepath.Join(tmpDir, testPattern))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(files).Should(HaveLen(3))
	})

	t.Run("should deduplicate glob results with memory precedence", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsOnDisk()
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")

		err := delegate.WriteFile(testFile, []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride(testFile, []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		files, err := ufs.Glob(filepath.Join(tmpDir, "*.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(files).Should(HaveLen(1))

		// Verify content is from memory
		content, err := ufs.ReadFile(testFile)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content)).Should(Equal(testContent1))
	})

	t.Run("should merge walk results", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()
		err := delegate.WriteFile("dir/file1.txt", []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride("dir/file2.txt", []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		var walked []string
		err = ufs.Walk("dir", func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				walked = append(walked, filepath.Base(path))
			}

			return nil
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(walked).Should(ConsistOf("file1.txt", "file2.txt"))
	})

	t.Run("should not allow RemoveAll", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		ufs, err := unionfs.NewBuilder(delegate).Build()
		g.Expect(err).ToNot(HaveOccurred())

		err = ufs.RemoveAll(testFile1)
		g.Expect(err).Should(HaveOccurred())
		g.Expect(err.Error()).Should(ContainSubstring("RemoveAll not supported"))
	})

	t.Run("should create directories in memory layer", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		ufs, err := unionfs.NewBuilder(delegate).Build()
		g.Expect(err).ToNot(HaveOccurred())

		err = ufs.MkdirAll("dir/subdir")
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(ufs.IsDir("dir/subdir")).Should(BeTrue())
		g.Expect(delegate.Exists("dir/subdir")).Should(BeFalse())
	})

	t.Run("should open files from both layers", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()
		err := delegate.WriteFile(testFile1, []byte(testContent2))
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride(testFile2, []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		// Open from delegate
		f1, err := ufs.Open(testFile1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(f1).ToNot(BeNil())

		// Open from memory
		f2, err := ufs.Open(testFile2)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(f2).ToNot(BeNil())
	})

	t.Run("should check IsDir in both layers", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()
		err := delegate.MkdirAll("delegatedir")
		g.Expect(err).ToNot(HaveOccurred())

		ufs, err := unionfs.NewBuilder(delegate).Build()
		g.Expect(err).ToNot(HaveOccurred())

		err = ufs.MkdirAll("memdir")
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(ufs.IsDir("delegatedir")).Should(BeTrue())
		g.Expect(ufs.IsDir("memdir")).Should(BeTrue())
		g.Expect(ufs.IsDir("nonexistent")).Should(BeFalse())
	})
}

func TestBuilder(t *testing.T) {

	t.Run("should build empty union FS", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		ufs, err := unionfs.NewBuilder(delegate).Build()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ufs).ToNot(BeNil())
	})

	t.Run("should add single override", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride(testFile1, []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		content, err := ufs.ReadFile(testFile1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content)).Should(Equal(testContent1))
	})

	t.Run("should add multiple overrides via WithOverride", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride(testFile1, []byte(testContent1)).
			WithOverride(testFile2, []byte(testContent2)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		content1, err := ufs.ReadFile(testFile1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content1)).Should(Equal(testContent1))

		content2, err := ufs.ReadFile(testFile2)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content2)).Should(Equal(testContent2))
	})

	t.Run("should add multiple overrides via WithOverrides", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		overrides := map[string][]byte{
			testFile1: []byte(testContent1),
			testFile2: []byte(testContent2),
		}

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverrides(overrides).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		content1, err := ufs.ReadFile(testFile1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content1)).Should(Equal(testContent1))

		content2, err := ufs.ReadFile(testFile2)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content2)).Should(Equal(testContent2))
	})

	t.Run("should chain multiple WithOverrides calls", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		overrides1 := map[string][]byte{
			testFile1: []byte(testContent1),
		}
		overrides2 := map[string][]byte{
			testFile2: []byte(testContent2),
		}

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverrides(overrides1).
			WithOverrides(overrides2).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(ufs.Exists(testFile1)).Should(BeTrue())
		g.Expect(ufs.Exists(testFile2)).Should(BeTrue())
	})

	t.Run("should handle override with nested paths", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		ufs, err := unionfs.NewBuilder(delegate).
			WithOverride("dir/subdir/file.txt", []byte(testContent1)).
			Build()
		g.Expect(err).ToNot(HaveOccurred())

		content, err := ufs.ReadFile("dir/subdir/file.txt")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content)).Should(Equal(testContent1))
	})

	t.Run("should return error on invalid write during build", func(t *testing.T) {
		g := NewWithT(t)
		delegate := filesys.MakeFsInMemory()

		// Create a file in delegate to cause conflict
		err := delegate.WriteFile("readonly", []byte("data"))
		g.Expect(err).ToNot(HaveOccurred())

		// Try to override with invalid path (empty filename in directory structure causes error)
		_, err = unionfs.NewBuilder(delegate).
			WithOverride("", []byte("invalid")).
			Build()
		g.Expect(err).Should(HaveOccurred())
	})
}
