package kustomize_test

import (
	"path/filepath"
	"testing"

	"github.com/k8s-manifest-kit/pkg/util/cache"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

func TestPluginOptions(t *testing.T) {

	t.Run("should accept FnPluginLoadingOptions", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithFnPluginLoadingOptions(kustomizetypes.FnPluginLoadingOptions{
				EnableExec: true,
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(2))
	})

	t.Run("should accept BuiltinPluginLoadingOptions", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithBuiltinPluginLoadingOptions(kustomizetypes.BploUseStaticallyLinked),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(2))
	})

	t.Run("should compose all plugin options", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithFnPluginLoadingOptions(kustomizetypes.FnPluginLoadingOptions{
				EnableExec: true,
			}),
			kustomize.WithBuiltinPluginLoadingOptions(kustomizetypes.BploUseStaticallyLinked),
			kustomize.WithManagedByLabel(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(2))

		for _, obj := range objects {
			g.Expect(obj.GetLabels()).To(HaveKey("app.kubernetes.io/managed-by"))
		}
	})
}

func TestLoadRestrictions(t *testing.T) {

	t.Run("should use default LoadRestrictionsRootOnly", func(t *testing.T) {
		g := NewWithT(t)
		parentDir := t.TempDir()
		childDir := filepath.Join(parentDir, "child")

		// Create configmap in parent dir
		writeFile(t, parentDir, "configmap.yaml", basicConfigMap)

		// Create kustomization in child that tries to reference parent
		writeFile(t, childDir, "kustomization.yaml", kustomizationWithParent)

		renderer, err := kustomize.New([]kustomize.Source{
			{Path: childDir},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Should fail because default is LoadRestrictionsRootOnly
		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).Should(HaveOccurred())
		g.Expect(err.Error()).Should(ContainSubstring("failed to run kustomize"))
	})

	t.Run("should allow LoadRestrictionsNone via renderer option", func(t *testing.T) {
		g := NewWithT(t)
		parentDir := t.TempDir()
		childDir := filepath.Join(parentDir, "child")

		// Create configmap in parent dir
		writeFile(t, parentDir, "configmap.yaml", basicConfigMap)

		// Create kustomization in child that references parent
		writeFile(t, childDir, "kustomization.yaml", kustomizationWithParent)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{Path: childDir},
			},
			kustomize.WithLoadRestrictions(kustomizetypes.LoadRestrictionsNone),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Should succeed with LoadRestrictionsNone
		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).Should(HaveLen(1))
		g.Expect(objects[0].GetKind()).Should(Equal("ConfigMap"))
	})

	t.Run("should allow per-Source LoadRestrictions override", func(t *testing.T) {
		g := NewWithT(t)
		parentDir := t.TempDir()
		child1Dir := filepath.Join(parentDir, "child1")
		child2Dir := filepath.Join(parentDir, "child2")

		// Create configmap in parent dir
		writeFile(t, parentDir, "configmap.yaml", basicConfigMap)

		// Create kustomization in child1 that references parent (will use None)
		writeFile(t, child1Dir, "kustomization.yaml", kustomizationWithParent)

		// Create basic kustomization in child2 (will use RootOnly)
		writeFile(t, child2Dir, "kustomization.yaml", basicKustomization)
		writeFile(t, child2Dir, "configmap.yaml", basicConfigMap)
		writeFile(t, child2Dir, "pod.yaml", basicPod)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{
					Path:             child1Dir,
					LoadRestrictions: kustomizetypes.LoadRestrictionsNone,
				},
				{
					Path:             child2Dir,
					LoadRestrictions: kustomizetypes.LoadRestrictionsRootOnly,
				},
			},
			kustomize.WithLoadRestrictions(kustomizetypes.LoadRestrictionsRootOnly),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Should succeed: child1 uses None, child2 uses RootOnly
		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).Should(HaveLen(3)) // 1 from child1, 2 from child2
	})

	t.Run("should respect Source override over renderer-wide default", func(t *testing.T) {
		g := NewWithT(t)
		parentDir := t.TempDir()
		childDir := filepath.Join(parentDir, "child")

		// Create configmap in parent dir
		writeFile(t, parentDir, "configmap.yaml", basicConfigMap)

		// Create kustomization in child that references parent
		writeFile(t, childDir, "kustomization.yaml", kustomizationWithParent)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{
					Path:             childDir,
					LoadRestrictions: kustomizetypes.LoadRestrictionsRootOnly,
				},
			},
			kustomize.WithLoadRestrictions(kustomizetypes.LoadRestrictionsNone),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Should fail because Source overrides to RootOnly
		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).Should(HaveOccurred())
		g.Expect(err.Error()).Should(ContainSubstring("failed to run kustomize"))
	})

	t.Run("should use custom cache key function", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		// Use FastCacheKey which ignores values
		renderer, err := kustomize.New([]kustomize.Source{
			{
				Path: dir,
				Values: kustomize.Values(map[string]string{
					"key1": "value1",
				}),
			},
		},
			kustomize.WithCache(
				cache.WithKeyFunc(func(key any) string {
					if spec, ok := key.(kustomize.KustomizationSpec); ok {
						return spec.Path
					}

					return cache.DefaultKeyFunc(key)
				}),
			),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// First render - cache miss
		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).To(HaveLen(2))

		// Second render with DIFFERENT values but using FastCacheKey
		// ignores values, so this should be a cache hit
		result2, err := renderer.Process(t.Context(), map[string]any{
			"key1": "different-value",
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).To(HaveLen(len(result1)))

		// Results should be identical despite different values (cache hit)
		for i := range result1 {
			g.Expect(result2[i]).To(Equal(result1[i]))
		}
	})
}
