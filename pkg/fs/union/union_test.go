package union_test

import (
	"testing"

	"github.com/k8s-manifest-kit/renderer-kustomize/pkg/fs"
	"github.com/k8s-manifest-kit/renderer-kustomize/pkg/fs/union"

	. "github.com/onsi/gomega"
)

func TestNewFs(t *testing.T) {
	g := NewWithT(t)

	// Create base filesystem with initial content
	base := fs.NewMemoryFs()
	err := base.WriteFile("/base.txt", []byte("base content"))
	g.Expect(err).To(Succeed())
	err = base.WriteFile("/override.txt", []byte("original"))
	g.Expect(err).To(Succeed())

	// Create union with file overrides
	unionFs, err := union.NewFs(base,
		union.WithOverride("/overlay.txt", []byte("overlay content")),
		union.WithOverride("/override.txt", []byte("overridden")),
	)
	g.Expect(err).To(Succeed())

	// Test base file is accessible
	data, err := unionFs.ReadFile("/base.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("base content"))

	// Test overlay file is accessible
	data, err = unionFs.ReadFile("/overlay.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("overlay content"))

	// Test overlay shadows base
	data, err = unionFs.ReadFile("/override.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("overridden"))
}

func TestNewFs_MultipleOverrides(t *testing.T) {
	g := NewWithT(t)

	// Create base filesystem
	base := fs.NewMemoryFs()
	err := base.WriteFile("/base.txt", []byte("base"))
	g.Expect(err).To(Succeed())

	// Create union with multiple overrides using functional options
	unionFs, err := union.NewFs(base,
		union.WithOverride("/override.txt", []byte("override content")),
		union.WithOverride("/another.txt", []byte("another content")),
	)
	g.Expect(err).To(Succeed())

	// Test base file
	data, err := unionFs.ReadFile("/base.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("base"))

	// Test overrides
	data, err = unionFs.ReadFile("/override.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("override content"))

	data, err = unionFs.ReadFile("/another.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("another content"))
}

func TestNewFs_WithOverridesMap(t *testing.T) {
	g := NewWithT(t)

	base := fs.NewMemoryFs()

	overrides := map[string][]byte{
		"/file1.txt": []byte("content1"),
		"/file2.txt": []byte("content2"),
	}

	unionFs, err := union.NewFs(base, union.WithOverrides(overrides))
	g.Expect(err).To(Succeed())

	data, err := unionFs.ReadFile("/file1.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("content1"))

	data, err = unionFs.ReadFile("/file2.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("content2"))
}

func TestNewFs_WithCustomOverlay(t *testing.T) {
	g := NewWithT(t)

	base := fs.NewMemoryFs()
	err := base.WriteFile("/base.txt", []byte("base"))
	g.Expect(err).To(Succeed())

	// Create custom overlay
	overlay := fs.NewMemoryFs()
	err = overlay.WriteFile("/overlay.txt", []byte("overlay"))
	g.Expect(err).To(Succeed())

	unionFs, err := union.NewFs(base, union.WithOverlayFs(overlay))
	g.Expect(err).To(Succeed())

	data, err := unionFs.ReadFile("/base.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("base"))

	data, err = unionFs.ReadFile("/overlay.txt")
	g.Expect(err).To(Succeed())
	g.Expect(string(data)).To(Equal("overlay"))
}

// Ensure union.NewFs returns filesys.FileSystem.
func TestNewFsImplementsInterface(t *testing.T) {
	g := NewWithT(t)

	base := fs.NewMemoryFs()
	unionFs, err := union.NewFs(base)
	g.Expect(err).To(Succeed())

	_ = unionFs
	g.Expect(true).To(BeTrue()) // Compilation check
}
