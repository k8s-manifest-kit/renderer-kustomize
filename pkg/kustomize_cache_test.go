package kustomize_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

// cacheIsolationLabel is the label used to identify which registered source
// produced an object when several sources render the same path.
const cacheIsolationLabel = "cache-isolation.k8s-manifest-kit/source"

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

// labelPostRenderer returns a source post-renderer that marks every object
// with a fixed label value, making the source's outputs identifiable in
// combined results.
func labelPostRenderer(value string) types.PostRenderer {
	return func(_ context.Context, objects []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
		for i := range objects {
			labels := objects[i].GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}

			labels[cacheIsolationLabel] = value
			objects[i].SetLabels(labels)
		}

		return objects, nil
	}
}

// constantKeyFunc always returns the same cache key, representing the most
// permissive possible custom key configuration.
func constantKeyFunc(_ any) string {
	return "constant-key"
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
