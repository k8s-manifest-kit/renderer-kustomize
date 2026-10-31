package kustomize_test

import (
	"path/filepath"
	"testing"

	"github.com/k8s-manifest-kit/engine/pkg/filter/meta/gvk"
	"github.com/k8s-manifest-kit/engine/pkg/transformer/meta/labels"
	jqmatcher "github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"

	corev1 "k8s.io/api/core/v1"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

func TestRenderer(t *testing.T) {

	t.Run("should render basic kustomization", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{
			{Path: dir},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(2))

		// Check that namespace prefix was applied
		g.Expect(objects[0].Object).To(Or(
			jqmatcher.Match(`.metadata.name == "test-configmap"`),
			jqmatcher.Match(`.metadata.name == "test-pod"`),
		))
	})

	t.Run("should render kustomization with overlay", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupOverlayKustomization(t)

		overlayDir := filepath.Join(dir, "overlay")
		renderer, err := kustomize.New([]kustomize.Source{
			{Path: overlayDir},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))

		// Check that overlay label was applied
		g.Expect(objects[0].Object).To(And(
			jqmatcher.Match(`.kind == "ConfigMap"`),
			jqmatcher.Match(`.metadata.labels["environment"] == "production"`),
		))
	})

	t.Run("should apply filters", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithFilter(gvk.Filter(corev1.SchemeGroupVersion.WithKind("ConfigMap"))),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))
		g.Expect(objects[0].GetKind()).To(Equal("ConfigMap"))
	})

	t.Run("should apply transformers", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithTransformer(labels.Set(map[string]string{
				"managed-by": "kustomize-renderer",
			})),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(2))

		for _, obj := range objects {
			g.Expect(obj.GetLabels()).To(HaveKeyWithValue("managed-by", "kustomize-renderer"))
		}
	})

	t.Run("should process multiple inputs", func(t *testing.T) {
		g := NewWithT(t)
		dir1 := setupBasicKustomization(t)
		dir2 := setupSecondKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{
			{Path: dir1},
			{Path: dir2},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(3))
	})

	t.Run("should return error for non-existent path", func(t *testing.T) {
		g := NewWithT(t)
		renderer, err := kustomize.New([]kustomize.Source{
			{Path: "/non/existent/path"},
		})
		g.Expect(err).ToNot(HaveOccurred())

		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to run kustomize"))
	})

	t.Run("should provide helpful error for missing kustomization file", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()
		// Don't create any kustomization file

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).Should(HaveOccurred())
		g.Expect(err.Error()).Should(ContainSubstring("expected one of"))
		g.Expect(err.Error()).Should(ContainSubstring("kustomization.yaml"))
	})

	t.Run("should apply kustomize labels and namespace", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupKustomizationWithLabelsAndNamespace(t)

		renderer, err := kustomize.New([]kustomize.Source{
			{Path: dir},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))

		g.Expect(objects[0].Object).To(And(
			jqmatcher.Match(`.metadata.namespace == "custom-namespace"`),
			jqmatcher.Match(`.metadata.labels["app"] == "myapp"`),
		))
	})
}

func TestNew(t *testing.T) {

	t.Run("should reject input without path", func(t *testing.T) {
		g := NewWithT(t)
		renderer, err := kustomize.New([]kustomize.Source{{}})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("path cannot be empty or whitespace-only"))
		g.Expect(renderer).To(BeNil())
	})

	t.Run("should reject input with whitespace-only path", func(t *testing.T) {
		g := NewWithT(t)
		renderer, err := kustomize.New([]kustomize.Source{{Path: "   "}})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("path cannot be empty or whitespace-only"))
		g.Expect(renderer).To(BeNil())
	})

	t.Run("should accept valid input", func(t *testing.T) {
		g := NewWithT(t)
		renderer, err := kustomize.New([]kustomize.Source{
			{Path: "/some/path"},
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(renderer).ToNot(BeNil())
	})
}
