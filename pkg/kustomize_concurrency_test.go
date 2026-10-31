package kustomize_test

import (
	"testing"

	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

// TestConcurrentRendering verifies that overlapping Process calls on one
// shared Kustomize renderer keep per-render virtual values overlays and
// per-source cached results independent under the race detector.
func TestConcurrentRendering(t *testing.T) {

	t.Run("cache disabled", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupValuesKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		runValueIsolationScenario(t.Context(), g, renderer, dir, types.RenderOriginLive)
	})

	t.Run("cache enabled with default keys", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupValuesKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithCache(),
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		runValueIsolationScenario(t.Context(), g, renderer, dir, types.RenderOriginCache)
	})

	t.Run("cache enabled with constant custom keys", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupValuesKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{Path: dir, PostRenderers: []types.PostRenderer{labelPostRenderer("alpha")}},
				{Path: dir, PostRenderers: []types.PostRenderer{labelPostRenderer("beta")}},
			},
			kustomize.WithCache(cache.WithKeyFunc(constantKeyFunc)),
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		runSourceIsolationScenario(t.Context(), g, renderer, dir)
	})
}
