package kustomize_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

const deprecatedKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namePrefix: test-

commonLabels:
  app: myapp

resources:
- configmap.yaml
`

const deprecatedBasesKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
- ../base

commonLabels:
  environment: prod
`

func TestWarningHandlers(t *testing.T) {

	t.Run("WarningIgnore should suppress all warnings", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupDeprecatedKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithWarningHandler(kustomize.WarningIgnore()),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Should succeed without any warnings
		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))
		g.Expect(objects[0].GetKind()).To(Equal("ConfigMap"))
	})

	t.Run("WarningLog should write warnings to buffer", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupDeprecatedKustomization(t)

		var buf bytes.Buffer
		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithWarningHandler(kustomize.WarningLog(&buf)),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))

		// Check that warnings were written
		output := buf.String()
		g.Expect(output).ToNot(BeEmpty())
		g.Expect(output).To(ContainSubstring("commonLabels"))
	})

	t.Run("WarningFail should return error when warnings present", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupDeprecatedKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithWarningHandler(kustomize.WarningFail()),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Should fail due to warnings
		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err).To(MatchError(kustomize.ErrKustomizeWarnings))
		g.Expect(err.Error()).To(ContainSubstring("commonLabels"))
	})

	t.Run("default behavior should log to stderr", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupDeprecatedKustomization(t)

		// Capture stderr
		oldStderr := os.Stderr
		r, w, err := os.Pipe()
		g.Expect(err).ToNot(HaveOccurred())
		os.Stderr = w

		done := make(chan string)
		go func() {
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			done <- buf.String()
		}()

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))

		// Restore stderr
		_ = w.Close()
		os.Stderr = oldStderr
		output := <-done

		// Default should log warnings to stderr
		g.Expect(output).To(ContainSubstring("commonLabels"))
	})

	t.Run("should handle multiple warnings", func(t *testing.T) {
		g := NewWithT(t)
		parentDir := t.TempDir()
		baseDir := filepath.Join(parentDir, "base")
		overlayDir := filepath.Join(parentDir, "overlay")

		// Create base with deprecated field
		writeFile(t, baseDir, "kustomization.yaml", deprecatedKustomization)
		writeFile(t, baseDir, "configmap.yaml", basicConfigMap)

		// Create overlay with bases (deprecated)
		writeFile(t, overlayDir, "kustomization.yaml", deprecatedBasesKustomization)

		var buf bytes.Buffer
		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: overlayDir}},
			kustomize.WithWarningHandler(kustomize.WarningLog(&buf)),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))

		// Should have warnings about both commonLabels and bases
		output := buf.String()
		g.Expect(output).To(ContainSubstring("bases"))
	})

	t.Run("custom handler should receive warnings", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupDeprecatedKustomization(t)

		var receivedWarnings []string
		customHandler := func(warnings []string) error {
			receivedWarnings = warnings

			return nil
		}

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithWarningHandler(customHandler),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))

		// Custom handler should have received warnings
		g.Expect(receivedWarnings).ToNot(BeEmpty())
		g.Expect(strings.Join(receivedWarnings, " ")).To(ContainSubstring("commonLabels"))
	})

	t.Run("should not call handler when no warnings", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		handlerCalled := false
		customHandler := func(_ []string) error {
			handlerCalled = true

			return nil
		}

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithWarningHandler(customHandler),
		)
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(2))

		// Handler should not be called for valid kustomizations
		g.Expect(handlerCalled).To(BeFalse())
	})
}

func setupDeprecatedKustomization(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "kustomization.yaml", deprecatedKustomization)
	writeFile(t, dir, "configmap.yaml", basicConfigMap)

	return dir
}
