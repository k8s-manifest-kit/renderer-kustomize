package kustomize_test

import (
	"errors"
	"testing"

	engine "github.com/k8s-manifest-kit/engine/pkg"
	"github.com/k8s-manifest-kit/engine/pkg/types"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

func TestNewEngine(t *testing.T) {

	t.Run("should create engine with Kustomize renderer", func(t *testing.T) {
		g := NewWithT(t)
		e, err := kustomize.NewEngine(kustomize.Source{
			Path: "../config/test/kustomizations/simple",
		})

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(e).ShouldNot(BeNil())
	})

	t.Run("should return error for invalid source", func(t *testing.T) {
		g := NewWithT(t)
		e, err := kustomize.NewEngine(kustomize.Source{
			Path: "", // Missing path
		})

		g.Expect(err).Should(HaveOccurred())
		g.Expect(e).Should(BeNil())
	})
}

// TestEngineTypedNilRenderer verifies that engine construction accepts the
// typed-nil built-in — Name is nil-safe — and that rendering returns the
// nil-receiver sentinel without panic.
func TestEngineTypedNilRenderer(t *testing.T) {
	g := NewWithT(t)

	var renderer *kustomize.Renderer

	e, err := engine.New(engine.WithRenderer(renderer))
	g.Expect(err).ToNot(HaveOccurred(), "engine construction accepts the typed-nil built-in")

	objects, err := e.Render(t.Context())

	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, types.ErrRendererNil)).To(BeTrue())
	g.Expect(objects).To(BeNil())
}
