package kustomize_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/k8s-manifest-kit/engine/pkg/filter/meta/gvk"
	"github.com/k8s-manifest-kit/engine/pkg/transformer/meta/labels"
	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"
	jqmatcher "github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"
	"github.com/rs/xid"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"

	corev1 "k8s.io/api/core/v1"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

const basicKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namePrefix: test-

resources:
- configmap.yaml
- pod.yaml
`

const basicConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: configmap
data:
  key: value
`

const basicPod = `
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
`

const baseKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- configmap.yaml
`

const baseConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  env: dev
`

const overlayKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

commonLabels:
  environment: production

resources:
- ../base

patches:
- patch: |-
    - op: replace
      path: /data/env
      value: prod
  target:
    kind: ConfigMap
`

const secondKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- service.yaml
`

const secondService = `
apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  ports:
  - port: 80
`

const labelsNamespaceKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: custom-namespace

commonLabels:
  app: myapp

resources:
- deployment.yaml
`

const labelsNamespaceDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: placeholder
  template:
    metadata:
      labels:
        app: placeholder
    spec:
      containers:
      - name: nginx
        image: nginx:latest
`

// Test constants for nested resources test.
const nestedResourcesKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- resources/configmap.yaml
- resources/configs/secret.yaml
`

const nestedConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: nested-config
data:
  location: nested
`

//nolint:gosec
const nestedSecret = `
apiVersion: v1
kind: Secret
metadata:
  name: nested-secret
type: Opaque
stringData:
  key: value
`

// Test constants for base/overlay annotations test.
const annotationsBaseKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- configmap.yaml
- deployment.yaml
`

const annotationsBaseConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: base-config
data:
  env: base
`

const annotationsBaseDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: app
        image: nginx:latest
`

const annotationsOverlayKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../base

commonLabels:
  overlay: "true"
`

// Test constants for multiple components test.
const componentsMainKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- components/frontend
- components/backend
`

const componentsFrontendKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- deployment.yaml
`

const componentsFrontendDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: frontend
  template:
    metadata:
      labels:
        app: frontend
    spec:
      containers:
      - name: frontend
        image: nginx:latest
`

const componentsBackendKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- deployment.yaml
- service.yaml
`

const componentsBackendDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
    spec:
      containers:
      - name: backend
        image: nginx:latest
`

const componentsBackendService = `
apiVersion: v1
kind: Service
metadata:
  name: backend
spec:
  selector:
    app: backend
  ports:
  - port: 80
`

// Test constants for LoadRestrictions test.
const kustomizationWithParent = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../configmap.yaml
`

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

// Helper functions to set up test kustomizations

func setupBasicKustomization(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "kustomization.yaml", basicKustomization)
	writeFile(t, dir, "configmap.yaml", basicConfigMap)
	writeFile(t, dir, "pod.yaml", basicPod)

	return dir
}

func setupOverlayKustomization(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "base/kustomization.yaml", baseKustomization)
	writeFile(t, dir, "base/configmap.yaml", baseConfigMap)
	writeFile(t, dir, "overlay/kustomization.yaml", overlayKustomization)

	return dir
}

func setupSecondKustomization(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "kustomization.yaml", secondKustomization)
	writeFile(t, dir, "service.yaml", secondService)

	return dir
}

func setupKustomizationWithLabelsAndNamespace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "kustomization.yaml", labelsNamespaceKustomization)
	writeFile(t, dir, "deployment.yaml", labelsNamespaceDeployment)

	return dir
}

func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}

	err := os.WriteFile(path, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValuesConfigMap(t *testing.T) {

	t.Run("should write values as ConfigMap", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		values := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		renderer, err := kustomize.New([]kustomize.Source{
			{
				Path:   dir,
				Values: kustomize.Values(values),
			},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Render should create values.yaml
		_, err = renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())

		// With virtual filesystem, values.yaml never touches disk
	})

	t.Run("should work without values", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{
			{Path: dir},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(HaveLen(2))

		// No values.yaml should be created
		valuesPath := filepath.Join(dir, "values.yaml")
		g.Expect(valuesPath).ToNot(BeAnExistingFile())
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
		)
		g.Expect(err).ToNot(HaveOccurred())

		// First render - cache miss
		result1, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result1).ToNot(BeEmpty())

		// Second render - cache hit (should be identical)
		result2, err := renderer.Process(t.Context(), nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result2).To(HaveLen(len(result1)))

		// Results should be equal
		for i := range result1 {
			g.Expect(result2[i]).To(Equal(result1[i]))
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

func BenchmarkKustomizeRenderWithoutCache(b *testing.B) {
	dir := b.TempDir()

	writeFileB(b, dir, "kustomization.yaml", basicKustomization)
	writeFileB(b, dir, "configmap.yaml", basicConfigMap)
	writeFileB(b, dir, "pod.yaml", basicPod)

	renderer, err := kustomize.New([]kustomize.Source{
		{
			Path: dir,
			Values: kustomize.Values(map[string]string{
				"key1": "value1",
				"key2": "value2",
			}),
		},
	})
	if err != nil {
		b.Fatalf("failed to create renderer: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := renderer.Process(b.Context(), nil)
		if err != nil {
			b.Fatalf("failed to render: %v", err)
		}
	}
}

func BenchmarkKustomizeRenderWithCache(b *testing.B) {
	dir := b.TempDir()

	writeFileB(b, dir, "kustomization.yaml", basicKustomization)
	writeFileB(b, dir, "configmap.yaml", basicConfigMap)
	writeFileB(b, dir, "pod.yaml", basicPod)

	renderer, err := kustomize.New(
		[]kustomize.Source{
			{
				Path: dir,
				Values: kustomize.Values(map[string]string{
					"key1": "value1",
					"key2": "value2",
				}),
			},
		},
		kustomize.WithCache(),
	)
	if err != nil {
		b.Fatalf("failed to create renderer: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := renderer.Process(b.Context(), nil)
		if err != nil {
			b.Fatalf("failed to render: %v", err)
		}
	}
}

func BenchmarkKustomizeRenderCacheMiss(b *testing.B) {
	dir := b.TempDir()

	writeFileB(b, dir, "kustomization.yaml", basicKustomization)
	writeFileB(b, dir, "configmap.yaml", basicConfigMap)
	writeFileB(b, dir, "pod.yaml", basicPod)

	renderer, err := kustomize.New(
		[]kustomize.Source{
			{
				Path: dir,
				Values: func(_ context.Context) (map[string]string, error) {
					return map[string]string{
						"key": xid.New().String(),
					}, nil
				},
			},
		},
		kustomize.WithCache(),
	)
	if err != nil {
		b.Fatalf("failed to create renderer: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := renderer.Process(b.Context(), nil)
		if err != nil {
			b.Fatalf("failed to render: %v", err)
		}
	}
}

// Helper for benchmarks.
func writeFileB(b *testing.B, dir string, name string, content string) {
	b.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0600)
	if err != nil {
		b.Fatal(err)
	}
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
