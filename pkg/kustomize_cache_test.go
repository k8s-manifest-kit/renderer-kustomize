// Renderer-level cache behavior: per-source output isolation and cache-hit
// behavior through kustomize.New and Process, including sources under
// different load restrictions. Unit tests of the cache constructor and its
// key wiring live in pkg/cache.

package kustomize_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"
	"github.com/rs/xid"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

// setupOutsideRootKustomization creates a kustomization in a child directory
// that references a file in its parent directory, so it only builds under
// LoadRestrictionsNone.
func setupOutsideRootKustomization(t *testing.T) string {
	t.Helper()

	parentDir := t.TempDir()
	childDir := filepath.Join(parentDir, "child")

	writeFile(t, parentDir, "configmap.yaml", basicConfigMap)
	writeFile(t, childDir, "kustomization.yaml", kustomizationWithParent)

	return childDir
}

func TestCacheSourceIsolation(t *testing.T) {

	t.Run("should not reuse a permissive source's cache for a restrictive source", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupOutsideRootKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{Path: dir, LoadRestrictions: kustomizetypes.LoadRestrictionsNone},
				{Path: dir, LoadRestrictions: kustomizetypes.LoadRestrictionsRootOnly},
			},
			kustomize.WithCache(),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// The LoadRestrictionsRootOnly source must build and fail on the
		// out-of-root reference instead of returning the cached objects of
		// the LoadRestrictionsNone source.
		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to run kustomize"))
	})

	t.Run("should not reuse a permissive source's cache for a source inheriting the renderer default", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupOutsideRootKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{Path: dir, LoadRestrictions: kustomizetypes.LoadRestrictionsNone},
				{Path: dir},
			},
			kustomize.WithCache(),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// The second source leaves LoadRestrictions unset, so it inherits the
		// renderer-wide LoadRestrictionsRootOnly default and must fail.
		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to run kustomize"))
	})

	t.Run("should not let a constant custom key bypass a restrictive source", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupOutsideRootKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{Path: dir, LoadRestrictions: kustomizetypes.LoadRestrictionsNone},
				{Path: dir, LoadRestrictions: kustomizetypes.LoadRestrictionsRootOnly},
			},
			kustomize.WithCache(cache.WithKeyFunc(constantKeyFunc)),
		)
		g.Expect(err).ToNot(HaveOccurred())

		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to run kustomize"))
	})

	t.Run("should keep same-source cache hits with a constant custom key", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithCache(cache.WithKeyFunc(constantKeyFunc)),
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).To(HaveLen(2))
		for i := range result1 {
			g.Expect(result1[i].GetAnnotations()).To(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginLive,
			))
		}

		result2, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).To(HaveLen(2))
		for i := range result2 {
			g.Expect(result2[i].GetAnnotations()).To(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginCache,
			))
		}
	})

	t.Run("should give same-path sources independent live-to-cache transitions", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{Path: dir, PostRenderers: []types.PostRenderer{labelPostRenderer("alpha")}},
				{Path: dir, PostRenderers: []types.PostRenderer{labelPostRenderer("beta")}},
			},
			kustomize.WithCache(),
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// First render: both sources must build live; neither may reuse the
		// other's result despite the identical path and values.
		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).To(HaveLen(4))
		for i := range result1 {
			g.Expect(result1[i].GetAnnotations()).To(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginLive,
			))
		}

		// Mutate returned objects to confirm later renders stay independent.
		result1[0].SetName("mutated-name")
		if data, ok := result1[0].Object["data"].(map[string]any); ok {
			data["mutated"] = "true"
		}

		// Second render: both sources hit their own cache, and each source's
		// post-renderers are reapplied to its cached objects.
		result2, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).To(HaveLen(4))

		sources := map[string]int{}
		for i := range result2 {
			g.Expect(result2[i].GetAnnotations()).To(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginCache,
			))
			g.Expect(result2[i].GetName()).ToNot(Equal("mutated-name"))

			source := result2[i].GetLabels()[cacheIsolationLabel]
			sources[source]++
		}

		g.Expect(sources).To(Equal(map[string]int{"alpha": 2, "beta": 2}))
	})
}

func TestCacheIntegration(t *testing.T) {

	t.Run("should cache identical renders", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{
			{
				Path: dir,
				Values: kustomize.Values(map[string]string{
					"key": "value",
				}),
			},
		},
			kustomize.WithCache(),
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// First render - cache miss
		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).ToNot(BeEmpty())
		for _, obj := range result1 {
			g.Expect(obj.GetAnnotations()).Should(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginLive,
			))
		}

		// Second render - cache hit
		result2, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).To(HaveLen(len(result1)))

		for _, obj := range result2 {
			g.Expect(obj.GetAnnotations()).Should(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginCache,
			))
		}

		// A warm hit is origin-only: the batch was stored before the live
		// origin annotation, so the hit re-serves the exact source fields and
		// hash bytes captured on the live render and only swaps the origin.
		for i := range result1 {
			live := result1[i].GetAnnotations()
			warm := result2[i].GetAnnotations()

			g.Expect(warm[types.AnnotationSourceType]).To(Equal("kustomize"))
			g.Expect(warm[types.AnnotationSourcePath]).To(Equal(live[types.AnnotationSourcePath]))
			g.Expect(warm[types.AnnotationSourceFile]).To(Equal(live[types.AnnotationSourceFile]))
			g.Expect(warm[types.AnnotationContentHash]).To(Equal(live[types.AnnotationContentHash]))
		}
	})

	t.Run("should miss cache on different values", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		callCount := 0
		dynamicValues := func(_ context.Context) (map[string]string, error) {
			callCount++

			return map[string]string{
				"key": xid.New().String(),
			}, nil
		}

		renderer, err := kustomize.New([]kustomize.Source{
			{
				Path:   dir,
				Values: dynamicValues,
			},
		},
			kustomize.WithCache(),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// First render
		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).ToNot(BeEmpty())

		// Second render with different values - cache miss
		result2, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).ToNot(BeEmpty())

		// Values function should be called twice (no cache hits)
		g.Expect(callCount).To(Equal(2))
	})

	t.Run("should work with cache disabled", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{
					Path: dir,
					Values: kustomize.Values(map[string]string{
						"key": "value",
					}),
				},
			},
		)
		g.Expect(err).ToNot(HaveOccurred())

		// First render
		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).ToNot(BeEmpty())

		// Second render - should work even without cache
		result2, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).To(HaveLen(len(result1)))
	})

	t.Run("should return clones from cache", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{
			{
				Path: dir,
				Values: kustomize.Values(map[string]string{
					"key": "value",
				}),
			},
		},
			kustomize.WithCache(),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// First render
		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).ToNot(BeEmpty())

		// Modify first result
		if len(result1) > 0 {
			result1[0].SetName("modified-name")
		}

		// Second render - should not be affected by modification
		result2, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).ToNot(BeEmpty())

		if len(result2) > 0 {
			g.Expect(result2[0].GetName()).ToNot(Equal("modified-name"))
		}
	})
}
