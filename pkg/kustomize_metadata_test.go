package kustomize_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	engine "github.com/k8s-manifest-kit/engine/pkg"
	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/k8s"
	"sigs.k8s.io/kustomize/api/resmap"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

// upstreamOriginKey is the annotation kustomize uses to track resource
// origins when originAnnotations build metadata is enabled.
const upstreamOriginKey = "config.kubernetes.io/origin"

// seededOriginKustomization is a plain kustomization without build metadata;
// the renderer injects originAnnotations itself when source annotations are
// enabled.
const seededOriginKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- configmap.yaml
`

// explicitOriginKustomization requests originAnnotations itself, so the
// upstream origin annotation is user-requested and must survive rendering.
const explicitOriginKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

buildMetadata:
- originAnnotations

resources:
- configmap.yaml
`

// seededOriginConfigMap ships its own annotations, including a kit-managed
// source.file, to distinguish preservation from explicit empty overwrites
// when the renderer applies conditional file provenance.
const seededOriginConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: origin-config
  annotations:
    existing: annotation
    manifests.k8s-manifests-kit/source.file: seeded/path.yaml
data:
  key: value
`

// engineCompareKustomization is a plain resource-only kustomization shared by
// both engine-comparison fixtures: no build metadata, values, generators,
// plugins, or transformations.
const engineCompareKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- configmap.yaml
`

// engineCompareFirstConfigMap is the first engine-comparison resource. Both
// renderers share the same type name, so the ordered outputs stay
// distinguishable by fixture identity rather than by renderer name.
const engineCompareFirstConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: engine-compare-first
  annotations:
    note: first
data:
  key: first-value
`

// engineCompareSecondConfigMap is the second engine-comparison resource.
const engineCompareSecondConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: engine-compare-second
  annotations:
    note: second
data:
  key: second-value
`

// reingestKustomization is the plain resource-only kustomization used by both
// re-ingest stages: no build metadata, values, generators, plugins, or
// transformations.
const reingestKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- configmap.yaml
`

// reingestConfigMap is the minimal first-stage re-ingest fixture: a stable
// name, string data, and one ordinary user annotation.
const reingestConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: reingest-config
  annotations:
    note: ordinary-user-annotation
data:
  key: value
`

// pluginTransformer adapts a function to the kustomize plugin seam.
type pluginTransformer func(resmap.ResMap) error

// Transform implements resmap.Transformer.
func (f pluginTransformer) Transform(m resmap.ResMap) error {
	return f(m)
}

// rewriteOriginAnnotation returns a kustomize plugin transformer that
// replaces the upstream origin annotation of every resource with the given
// raw value, or removes it when the value is empty.
func rewriteOriginAnnotation(value string) resmap.Transformer {
	return pluginTransformer(func(m resmap.ResMap) error {
		for _, res := range m.Resources() {
			annotations := res.GetAnnotations()
			if value == "" {
				delete(annotations, upstreamOriginKey)
			} else {
				annotations[upstreamOriginKey] = value
			}

			if err := res.SetAnnotations(annotations); err != nil {
				return fmt.Errorf("failed to rewrite origin annotations: %w", err)
			}
		}

		return nil
	})
}

// setupSeededKustomization writes a kustomization directory whose single
// ConfigMap carries seeded source.file metadata.
func setupSeededKustomization(t *testing.T, kustomization string) string {
	t.Helper()

	dir := t.TempDir()
	writeFile(t, dir, "kustomization.yaml", kustomization)
	writeFile(t, dir, "configmap.yaml", seededOriginConfigMap)

	return dir
}

// renderKustomization renders the directory with the given options.
func renderKustomization(t *testing.T, dir string, opts ...kustomize.RendererOption) []unstructured.Unstructured {
	t.Helper()

	g := NewWithT(t)

	renderer, err := kustomize.New([]kustomize.Source{{Path: dir}}, opts...)
	g.Expect(err).ToNot(HaveOccurred())

	objects, err := renderer.Process(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred())

	return objects
}

// deepCopyObjects returns an independent deep copy of an ordered object
// batch, so later renders cannot mutate a retained snapshot.
func deepCopyObjects(objects []unstructured.Unstructured) []unstructured.Unstructured {
	copies := make([]unstructured.Unstructured, len(objects))
	for i := range objects {
		copies[i] = *objects[i].DeepCopy()
	}

	return copies
}

// hashOver returns the content hash of the object with its content-hash and
// render-origin annotations replaced by the given upstream origin annotation
// value (or with the upstream annotation removed when the value is empty),
// reconstructing the exact hash input at conversion time.
func hashOver(t *testing.T, object unstructured.Unstructured, upstreamOrigin string) string {
	t.Helper()

	expected := object.DeepCopy()
	annotations := maps.Clone(object.GetAnnotations())
	g := NewWithT(t)
	g.Expect(annotations).NotTo(BeNil())

	delete(annotations, types.AnnotationContentHash)
	delete(annotations, types.AnnotationRenderOrigin)
	if upstreamOrigin == "" {
		delete(annotations, upstreamOriginKey)
	} else {
		annotations[upstreamOriginKey] = upstreamOrigin
	}

	expected.SetAnnotations(annotations)

	return k8s.ContentHash(expected)
}

// TestOriginAnnotationApplication characterizes how upstream origin
// annotations, seeded source.file metadata, content hashes, and origin
// cleanup interact during conversion.
func TestOriginAnnotationApplication(t *testing.T) {

	t.Run("explicitly requested origin remains and is hashed with the object", func(t *testing.T) {
		g := NewWithT(t)

		dir := setupSeededKustomization(t, explicitOriginKustomization)
		objects := renderKustomization(t, dir, kustomize.WithSourceAnnotations(true))

		g.Expect(objects).To(HaveLen(1))

		annotations := objects[0].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourcePath, dir))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceFile, "configmap.yaml"))
		g.Expect(annotations).To(HaveKeyWithValue("existing", "annotation"))
		g.Expect(annotations).To(HaveKey(upstreamOriginKey))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationRenderOrigin, types.RenderOriginLive))

		upstream := annotations[upstreamOriginKey]
		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationContentHash,
			hashOver(t, objects[0], upstream),
		))
	})

	t.Run("injected origin is hashed before cleanup and then removed", func(t *testing.T) {
		g := NewWithT(t)

		// The upstream origin annotation value is a relative path, so the
		// value kustomize generates for the explicit render is exactly the
		// value that was present while the injected render was hashed.
		explicit := renderKustomization(
			t,
			setupSeededKustomization(t, explicitOriginKustomization),
			kustomize.WithSourceAnnotations(true),
		)
		upstream := explicit[0].GetAnnotations()[upstreamOriginKey]
		g.Expect(upstream).NotTo(BeEmpty())

		injectedDir := setupSeededKustomization(t, seededOriginKustomization)
		injected := renderKustomization(t, injectedDir, kustomize.WithSourceAnnotations(true))

		g.Expect(explicit).To(HaveLen(1))
		g.Expect(injected).To(HaveLen(1))

		annotations := injected[0].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourcePath, injectedDir))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceFile, "configmap.yaml"))
		g.Expect(annotations).NotTo(HaveKey(upstreamOriginKey))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationRenderOrigin, types.RenderOriginLive))

		// The hash was computed while the injected origin annotation was
		// still present, before the cleanup removed it.
		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationContentHash,
			hashOver(t, injected[0], upstream),
		))
	})

	t.Run("nil origin preserves seeded file metadata", func(t *testing.T) {
		g := NewWithT(t)

		dir := setupSeededKustomization(t, seededOriginKustomization)
		objects := renderKustomization(t, dir,
			kustomize.WithSourceAnnotations(true),
			kustomize.WithPlugin(rewriteOriginAnnotation("")),
		)

		g.Expect(objects).To(HaveLen(1))

		annotations := objects[0].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourcePath, dir))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceFile, "seeded/path.yaml"))
		g.Expect(annotations).To(HaveKeyWithValue("existing", "annotation"))
		g.Expect(annotations).NotTo(HaveKey(upstreamOriginKey))

		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationContentHash,
			hashOver(t, objects[0], ""),
		))
	})

	t.Run("malformed origin preserves seeded file metadata", func(t *testing.T) {
		g := NewWithT(t)

		const malformed = "[not, an, origin]"

		dir := setupSeededKustomization(t, seededOriginKustomization)
		objects := renderKustomization(t, dir,
			kustomize.WithSourceAnnotations(true),
			kustomize.WithPlugin(rewriteOriginAnnotation(malformed)),
		)

		g.Expect(objects).To(HaveLen(1))

		annotations := objects[0].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourcePath, dir))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceFile, "seeded/path.yaml"))
		g.Expect(annotations).To(HaveKeyWithValue("existing", "annotation"))
		g.Expect(annotations).NotTo(HaveKey(upstreamOriginKey))

		// The malformed annotation was still part of the hash input.
		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationContentHash,
			hashOver(t, objects[0], malformed),
		))
	})

	t.Run("parsed origin with empty path overwrites seeded file with an empty value", func(t *testing.T) {
		g := NewWithT(t)

		const emptyPathOrigin = "repo: https://example.com/repo"

		dir := setupSeededKustomization(t, seededOriginKustomization)
		objects := renderKustomization(t, dir,
			kustomize.WithSourceAnnotations(true),
			kustomize.WithPlugin(rewriteOriginAnnotation(emptyPathOrigin)),
		)

		g.Expect(objects).To(HaveLen(1))

		annotations := objects[0].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourcePath, dir))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceFile, ""))
		g.Expect(annotations).To(HaveKeyWithValue("existing", "annotation"))
		g.Expect(annotations).NotTo(HaveKey(upstreamOriginKey))

		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationContentHash,
			hashOver(t, objects[0], emptyPathOrigin),
		))
	})

	t.Run("source annotations without content hash leave no hash key", func(t *testing.T) {
		g := NewWithT(t)

		objects := renderKustomization(
			t,
			setupSeededKustomization(t, seededOriginKustomization),
			kustomize.WithSourceAnnotations(true),
			kustomize.WithContentHash(false),
		)

		g.Expect(objects).To(HaveLen(1))

		annotations := objects[0].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
		g.Expect(annotations).To(HaveKey(types.AnnotationSourcePath))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceFile, "configmap.yaml"))
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationRenderOrigin, types.RenderOriginLive))
		g.Expect(annotations).NotTo(HaveKey(types.AnnotationContentHash))
	})
}

// TestEngineCompositionHashPreservation pins that two real Kustomize
// renderers produce exactly the same objects through the orchestration
// engine as they do when processed directly — content hashes, source
// annotations, and render origins included — on every uncached render, and
// that engine runs never mutate the retained direct baselines. Hashes are
// compared as rendered, never recomputed over the final output or normalized
// by stripping upstream annotations.
func TestEngineCompositionHashPreservation(t *testing.T) {
	g := NewWithT(t)

	firstDir := t.TempDir()
	writeFile(t, firstDir, "kustomization.yaml", engineCompareKustomization)
	writeFile(t, firstDir, "configmap.yaml", engineCompareFirstConfigMap)

	secondDir := t.TempDir()
	writeFile(t, secondDir, "kustomization.yaml", engineCompareKustomization)
	writeFile(t, secondDir, "configmap.yaml", engineCompareSecondConfigMap)

	first, err := kustomize.New(
		[]kustomize.Source{{Path: firstDir}},
		kustomize.WithSourceAnnotations(true),
	)
	g.Expect(err).ToNot(HaveOccurred(), "first renderer construction")

	second, err := kustomize.New(
		[]kustomize.Source{{Path: secondDir}},
		kustomize.WithSourceAnnotations(true),
	)
	g.Expect(err).ToNot(HaveOccurred(), "second renderer construction")

	baselineFirst, err := first.Process(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred(), "direct first baseline render")

	baselineSecond, err := second.Process(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred(), "direct second baseline render")

	g.Expect(baselineFirst).To(HaveLen(1))
	g.Expect(baselineSecond).To(HaveLen(1))
	g.Expect(baselineFirst[0].GetName()).To(Equal("engine-compare-first"))
	g.Expect(baselineSecond[0].GetName()).To(Equal("engine-compare-second"))

	baselines := []struct {
		objects []unstructured.Unstructured
		dir     string
	}{
		{objects: baselineFirst, dir: firstDir},
		{objects: baselineSecond, dir: secondDir},
	}

	for i, baseline := range baselines {
		annotations := baseline.objects[0].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationSourceType,
			"kustomize",
		), "baseline %d source type", i)
		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationSourcePath,
			baseline.dir,
		), "baseline %d source path", i)
		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationRenderOrigin,
			types.RenderOriginLive,
		), "baseline %d render origin", i)
		g.Expect(annotations).To(HaveKeyWithValue(
			types.AnnotationContentHash,
			MatchRegexp("^sha256:[0-9a-f]{64}$"),
		), "baseline %d content hash", i)
	}

	expected := append(deepCopyObjects(baselineFirst), deepCopyObjects(baselineSecond)...)
	firstSnapshot := deepCopyObjects(baselineFirst)
	secondSnapshot := deepCopyObjects(baselineSecond)

	e, err := engine.New(engine.WithRenderer(first), engine.WithRenderer(second))
	g.Expect(err).ToNot(HaveOccurred(), "engine construction")

	for pass := 1; pass <= 2; pass++ {
		result, renderErr := e.Render(t.Context())
		g.Expect(renderErr).ToNot(HaveOccurred(), "engine render pass %d", pass)
		g.Expect(result).To(HaveLen(2), "engine render pass %d", pass)
		g.Expect(result).To(Equal(expected), "engine render pass %d full result", pass)
	}

	g.Expect(baselineFirst).To(
		Equal(firstSnapshot),
		"first direct baseline snapshot after engine renders",
	)
	g.Expect(baselineSecond).To(
		Equal(secondSnapshot),
		"second direct baseline snapshot after engine renders",
	)
}

// TestAnnotatedResourceReIngestion pins the exact one-hash behavior when an
// actual annotated first-stage output is re-ingested as a plain resource: the
// second hash covers the first hash and differs from it, the fixed
// second-stage input renders deterministically across two uncached passes,
// and no source or origin metadata is injected on re-ingest.
func TestAnnotatedResourceReIngestion(t *testing.T) {
	g := NewWithT(t)

	firstDir := t.TempDir()
	writeFile(t, firstDir, "kustomization.yaml", reingestKustomization)
	writeFile(t, firstDir, "configmap.yaml", reingestConfigMap)

	first, err := kustomize.New([]kustomize.Source{{Path: firstDir}})
	g.Expect(err).ToNot(HaveOccurred(), "first-stage renderer construction")

	firstObjects, err := first.Process(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred(), "first-stage render")
	g.Expect(firstObjects).To(HaveLen(1))

	firstHash := firstObjects[0].GetAnnotations()[types.AnnotationContentHash]
	g.Expect(firstHash).NotTo(BeEmpty(), "first-stage content hash annotation")

	frozen := firstObjects[0].DeepCopy()

	resourceBytes, err := json.Marshal(frozen.Object)
	g.Expect(err).ToNot(HaveOccurred(), "marshaling the annotated first output")

	secondDir := t.TempDir()
	writeFile(t, secondDir, "kustomization.yaml", reingestKustomization)
	writeFile(t, secondDir, "configmap.yaml", string(resourceBytes))

	expectedHash := k8s.ContentHash(frozen.DeepCopy())

	expected := frozen.DeepCopy()
	expectedAnnotations := maps.Clone(expected.GetAnnotations())
	expectedAnnotations[types.AnnotationContentHash] = expectedHash
	expected.SetAnnotations(expectedAnnotations)

	g.Expect(expectedHash).NotTo(Equal(firstHash), "second hash must cover the first hash")

	second, err := kustomize.New([]kustomize.Source{{Path: secondDir}})
	g.Expect(err).ToNot(HaveOccurred(), "second-stage renderer construction")

	for pass := 1; pass <= 2; pass++ {
		objects, renderErr := second.Process(t.Context(), nil)
		g.Expect(renderErr).ToNot(HaveOccurred(), "re-ingest render pass %d", pass)
		g.Expect(objects).To(HaveLen(1), "re-ingest render pass %d", pass)
		g.Expect(objects[0]).To(Equal(*expected), "full re-ingested object, pass %d", pass)
	}

	g.Expect(firstObjects[0]).To(Equal(*frozen), "first-stage output snapshot")

	//nolint:gosec // G304: the path is a test-owned temp-dir fixture, not user input
	stored, err := os.ReadFile(filepath.Join(secondDir, "configmap.yaml"))
	g.Expect(err).ToNot(HaveOccurred(), "reading back the serialized resource")
	g.Expect(stored).To(Equal(resourceBytes), "serialized resource bytes unchanged")
}

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
