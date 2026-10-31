package kustomize_test

import (
	"path/filepath"
	"testing"

	"github.com/k8s-manifest-kit/engine/pkg/types"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

func TestSourceAnnotations(t *testing.T) {

	t.Run("should add source annotations when enabled", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "kustomization.yaml", basicKustomization)
		writeFile(t, dir, "configmap.yaml", basicConfigMap)
		writeFile(t, dir, "pod.yaml", basicPod)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{Path: dir},
			},
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).ToNot(BeEmpty())

		// Verify all objects have source annotations
		for _, obj := range objects {
			annotations := obj.GetAnnotations()
			g.Expect(annotations).Should(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
			g.Expect(annotations).Should(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginLive,
			))
			g.Expect(annotations).Should(HaveKeyWithValue(types.AnnotationSourcePath, dir))
			// Kustomize renderer should have file annotation with relative path
			g.Expect(annotations).Should(HaveKey(types.AnnotationSourceFile))
			g.Expect(annotations[types.AnnotationSourceFile]).ShouldNot(BeEmpty())
			// File should be one of: configmap.yaml or pod.yaml
			g.Expect(annotations[types.AnnotationSourceFile]).Should(
				Or(
					Equal("configmap.yaml"),
					Equal("pod.yaml"),
				),
			)
		}
	})

	t.Run("should not add source annotations when disabled", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "kustomization.yaml", basicKustomization)
		writeFile(t, dir, "configmap.yaml", basicConfigMap)
		writeFile(t, dir, "pod.yaml", basicPod)

		renderer, err := kustomize.New([]kustomize.Source{
			{Path: dir},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).ToNot(BeEmpty())

		// Verify no source annotations are present
		for _, obj := range objects {
			annotations := obj.GetAnnotations()
			g.Expect(annotations).ShouldNot(HaveKey(types.AnnotationSourceType))
			g.Expect(annotations).ShouldNot(HaveKey(types.AnnotationSourcePath))
			// g.Expect(annotations).ShouldNot(HaveKey(types.AnnotationSourceFile))
		}
	})

	t.Run("should annotate nested resources with relative paths", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "kustomization.yaml", nestedResourcesKustomization)
		writeFile(t, dir, "resources/configmap.yaml", nestedConfigMap)
		writeFile(t, dir, "resources/configs/secret.yaml", nestedSecret)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).Should(HaveLen(2))

		// Verify annotations include nested paths
		foundConfigMap := false
		foundSecret := false

		for _, obj := range objects {
			annotations := obj.GetAnnotations()
			g.Expect(annotations).Should(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
			g.Expect(annotations).Should(HaveKeyWithValue(types.AnnotationSourcePath, dir))
			g.Expect(annotations).Should(HaveKey(types.AnnotationSourceFile))

			sourceFile := annotations[types.AnnotationSourceFile]
			switch sourceFile {
			case "resources/configmap.yaml":
				foundConfigMap = true
				g.Expect(obj.GetKind()).Should(Equal("ConfigMap"))
			case "resources/configs/secret.yaml":
				foundSecret = true
				g.Expect(obj.GetKind()).Should(Equal("Secret"))
			default:
				// Other source files are ignored in this test
			}
		}

		g.Expect(foundConfigMap).Should(BeTrue(), "should find ConfigMap with nested path")
		g.Expect(foundSecret).Should(BeTrue(), "should find Secret with deeply nested path")
	})

	t.Run("should annotate resources from base kustomization", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "base/kustomization.yaml", annotationsBaseKustomization)
		writeFile(t, dir, "base/configmap.yaml", annotationsBaseConfigMap)
		writeFile(t, dir, "base/deployment.yaml", annotationsBaseDeployment)
		writeFile(t, dir, "overlay/kustomization.yaml", annotationsOverlayKustomization)

		overlayDir := filepath.Join(dir, "overlay")

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: overlayDir}},
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).Should(HaveLen(2))

		// Verify all objects have source annotations pointing to overlay
		for _, obj := range objects {
			annotations := obj.GetAnnotations()
			g.Expect(annotations).Should(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
			g.Expect(annotations).Should(HaveKeyWithValue(types.AnnotationSourcePath, overlayDir))
			g.Expect(annotations).Should(HaveKey(types.AnnotationSourceFile))

			// Source files should reference the base directory
			sourceFile := annotations[types.AnnotationSourceFile]
			g.Expect(sourceFile).Should(
				Or(
					Equal("../base/configmap.yaml"),
					Equal("../base/deployment.yaml"),
				),
				"source file should reference base directory",
			)

			// Verify overlay label was applied
			g.Expect(obj.GetLabels()).Should(HaveKeyWithValue("overlay", "true"))
		}
	})

	t.Run("should annotate resources from multiple included kustomizations", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "components/frontend/kustomization.yaml", componentsFrontendKustomization)
		writeFile(t, dir, "components/frontend/deployment.yaml", componentsFrontendDeployment)
		writeFile(t, dir, "components/backend/kustomization.yaml", componentsBackendKustomization)
		writeFile(t, dir, "components/backend/deployment.yaml", componentsBackendDeployment)
		writeFile(t, dir, "components/backend/service.yaml", componentsBackendService)
		writeFile(t, dir, "kustomization.yaml", componentsMainKustomization)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).Should(HaveLen(3))

		// Map to track found resources
		foundResources := make(map[string]string)

		for _, obj := range objects {
			annotations := obj.GetAnnotations()
			g.Expect(annotations).Should(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
			g.Expect(annotations).Should(HaveKeyWithValue(types.AnnotationSourcePath, dir))
			g.Expect(annotations).Should(HaveKey(types.AnnotationSourceFile))

			sourceFile := annotations[types.AnnotationSourceFile]
			key := obj.GetKind() + "/" + obj.GetName()
			foundResources[key] = sourceFile
		}

		// Verify we found all expected resources with correct source files
		g.Expect(foundResources).Should(HaveKeyWithValue("Deployment/frontend", "components/frontend/deployment.yaml"))
		g.Expect(foundResources).Should(HaveKeyWithValue("Deployment/backend", "components/backend/deployment.yaml"))
		g.Expect(foundResources).Should(HaveKeyWithValue("Service/backend", "components/backend/service.yaml"))
	})
}

func TestContentHash(t *testing.T) {

	t.Run("should add content hash annotation by default", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "kustomization.yaml", basicKustomization)
		writeFile(t, dir, "configmap.yaml", basicConfigMap)
		writeFile(t, dir, "pod.yaml", basicPod)

		renderer, err := kustomize.New([]kustomize.Source{
			{Path: dir},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).ToNot(BeEmpty())

		for _, obj := range objects {
			annotations := obj.GetAnnotations()
			g.Expect(annotations).Should(HaveKey(types.AnnotationContentHash))
			g.Expect(annotations[types.AnnotationContentHash]).Should(MatchRegexp("^sha256:[0-9a-f]{64}$"))
		}
	})

	t.Run("should not add content hash when disabled", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "kustomization.yaml", basicKustomization)
		writeFile(t, dir, "configmap.yaml", basicConfigMap)
		writeFile(t, dir, "pod.yaml", basicPod)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithContentHash(false),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).ToNot(BeEmpty())

		for _, obj := range objects {
			annotations := obj.GetAnnotations()
			g.Expect(annotations).ShouldNot(HaveKey(types.AnnotationContentHash))
		}
	})

	t.Run("different objects should have different hashes", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "kustomization.yaml", basicKustomization)
		writeFile(t, dir, "configmap.yaml", basicConfigMap)
		writeFile(t, dir, "pod.yaml", basicPod)

		renderer, err := kustomize.New([]kustomize.Source{
			{Path: dir},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(2))

		hash0 := objects[0].GetAnnotations()[types.AnnotationContentHash]
		hash1 := objects[1].GetAnnotations()[types.AnnotationContentHash]
		g.Expect(hash0).ShouldNot(Equal(hash1))
	})

	t.Run("hash should change when content changes", func(t *testing.T) {
		g := NewWithT(t)

		dir1 := t.TempDir()
		writeFile(t, dir1, "kustomization.yaml", basicKustomization)
		writeFile(t, dir1, "configmap.yaml", basicConfigMap)
		writeFile(t, dir1, "pod.yaml", basicPod)

		r1, err := kustomize.New([]kustomize.Source{{Path: dir1}})
		g.Expect(err).ToNot(HaveOccurred())
		objects1, err := r1.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects1).ToNot(BeEmpty())

		// Use a different ConfigMap with different data
		const altConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: configmap
data:
  key: different-value
`
		dir2 := t.TempDir()
		writeFile(t, dir2, "kustomization.yaml", basicKustomization)
		writeFile(t, dir2, "configmap.yaml", altConfigMap)
		writeFile(t, dir2, "pod.yaml", basicPod)

		r2, err := kustomize.New([]kustomize.Source{{Path: dir2}})
		g.Expect(err).ToNot(HaveOccurred())
		objects2, err := r2.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects2).ToNot(BeEmpty())

		// Find the ConfigMap in both results and compare hashes
		var hash1, hash2 string
		for _, obj := range objects1 {
			if obj.GetKind() == "ConfigMap" {
				hash1 = obj.GetAnnotations()[types.AnnotationContentHash]
			}
		}
		for _, obj := range objects2 {
			if obj.GetKind() == "ConfigMap" {
				hash2 = obj.GetAnnotations()[types.AnnotationContentHash]
			}
		}

		g.Expect(hash1).ShouldNot(BeEmpty())
		g.Expect(hash2).ShouldNot(BeEmpty())
		g.Expect(hash1).ShouldNot(Equal(hash2))
	})

	t.Run("cached results should preserve content hash", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		writeFile(t, dir, "kustomization.yaml", basicKustomization)
		writeFile(t, dir, "configmap.yaml", basicConfigMap)
		writeFile(t, dir, "pod.yaml", basicPod)

		renderer, err := kustomize.New(
			[]kustomize.Source{
				{
					Path:   dir,
					Values: kustomize.Values(map[string]string{"key": "value"}),
				},
			},
			kustomize.WithCache(),
		)
		g.Expect(err).ToNot(HaveOccurred())

		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).ToNot(BeEmpty())

		result2, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).To(HaveLen(len(result1)))

		for i := range result1 {
			hash1 := result1[i].GetAnnotations()[types.AnnotationContentHash]
			hash2 := result2[i].GetAnnotations()[types.AnnotationContentHash]
			g.Expect(hash1).ShouldNot(BeEmpty())
			g.Expect(hash1).Should(Equal(hash2))
		}
	})
}

func TestManagedByLabel(t *testing.T) {

	t.Run("should add managed-by label when enabled", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithManagedByLabel(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).ToNot(BeEmpty())

		for _, obj := range objects {
			g.Expect(obj.GetLabels()).To(HaveKey("app.kubernetes.io/managed-by"))
		}
	})

	t.Run("should not add managed-by label by default", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).ToNot(BeEmpty())

		for _, obj := range objects {
			g.Expect(obj.GetLabels()).ToNot(HaveKey("app.kubernetes.io/managed-by"))
		}
	})
}
