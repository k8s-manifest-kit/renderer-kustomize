package kustomize_test

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"

	"github.com/k8s-manifest-kit/engine/pkg/render"
	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

// secretStringer implements fmt.Stringer to prove that Stringer objects are
// rejected as values rather than formatted through their String method.
type secretStringer struct{}

func (secretStringer) String() string { return "secret-stringer-output" }

func TestRenderTimeScalarValues(t *testing.T) {

	t.Run("should expose every supported scalar type through the values ConfigMap", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupValuesKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), types.Values{
			"string":  "text",
			"bool":    true,
			"int":     int(42),
			"int8":    int8(-8),
			"int16":   int16(16),
			"int32":   int32(-32),
			"int64":   int64(64),
			"uint":    uint(7),
			"uint8":   uint8(8),
			"uint16":  uint16(16),
			"uint32":  uint32(32),
			"uint64":  uint64(64),
			"float32": float32(2.5),
			"float64": 1.5,
		})
		g.Expect(err).ToNot(HaveOccurred())

		valuesConfigMap := findValuesConfigMap(objects)
		g.Expect(valuesConfigMap).ToNot(BeNil(), "values ConfigMap should be rendered")

		// Scalars keep their fmt %v formatting: floats use the shortest
		// representation that round-trips, so 2.5 stays "2.5".
		g.Expect(valuesConfigMap.Object["data"]).To(Equal(map[string]any{
			"string":  "text",
			"bool":    "true",
			"int":     "42",
			"int8":    "-8",
			"int16":   "16",
			"int32":   "-32",
			"int64":   "64",
			"uint":    "7",
			"uint8":   "8",
			"uint16":  "16",
			"uint32":  "32",
			"uint64":  "64",
			"float32": "2.5",
			"float64": "1.5",
		}))
	})

	t.Run("should format numeric edge cases with the shortest round-trip representation", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupValuesKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), types.Values{
			"whole":    float64(3),
			"large":    float64(100000),
			"huge":     float64(1e21),
			"negative": float64(-0.5),
			"signed":   int(-5),
		})
		g.Expect(err).ToNot(HaveOccurred())

		valuesConfigMap := findValuesConfigMap(objects)
		g.Expect(valuesConfigMap).ToNot(BeNil())

		g.Expect(valuesConfigMap.Object["data"]).To(Equal(map[string]any{
			"whole":    "3",
			"large":    "100000",
			"huge":     "1e+21",
			"negative": "-0.5",
			"signed":   "-5",
		}))
	})

	t.Run("should override source values with render-time values", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupValuesKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{
			{
				Path: dir,
				Values: kustomize.Values(map[string]string{
					"shared":      "source",
					"source-only": "source",
				}),
			},
		})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := renderer.Process(t.Context(), types.Values{
			"shared":      "render-time",
			"render-only": "render-time",
		})
		g.Expect(err).ToNot(HaveOccurred())

		valuesConfigMap := findValuesConfigMap(objects)
		g.Expect(valuesConfigMap).ToNot(BeNil())

		// Render-time values take precedence over source values, and keys
		// from either side are preserved.
		g.Expect(valuesConfigMap.Object["data"]).To(Equal(map[string]any{
			"shared":      "render-time",
			"source-only": "source",
			"render-only": "render-time",
		}))
	})

	t.Run("should keep the values ConfigMap virtual", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupValuesKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		_, err = renderer.Process(t.Context(), types.Values{"key": "value"})
		g.Expect(err).ToNot(HaveOccurred())

		// values.yaml is served through the in-memory overlay and never
		// written to the source directory.
		g.Expect(filepath.Join(dir, "values.yaml")).ToNot(BeAnExistingFile())
	})
}

func TestRenderTimeValueValidation(t *testing.T) {

	t.Run("should reject unsupported values with the offending key and type", func(t *testing.T) {
		testCases := []struct {
			name     string
			value    any
			typeName string
		}{
			{"untyped nil", nil, "null"},
			{"typed nil pointer", (*int)(nil), "*int"},
			{"nested map", map[string]any{"a": "1"}, "map[string]interface {}"},
			{"untyped slice", []any{"a", "b"}, "[]interface {}"},
			{"typed string slice", []string{"a"}, "[]string"},
			{"typed int slice", []int{1}, "[]int"},
			{"pointer to string", new(string), "*string"},
			{"struct", secretStringer{}, "kustomize_test.secretStringer"},
			{"function", func() {}, "func()"},
			{"complex number", complex(1, 2), "complex128"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				g := NewWithT(t)
				dir := setupBasicKustomization(t)

				renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
				g.Expect(err).ToNot(HaveOccurred())

				_, err = renderer.Process(t.Context(), types.Values{"offending": tc.value})
				g.Expect(err).To(HaveOccurred())

				// The error names the source path, the offending key, and the
				// supported types, without dumping the value itself.
				g.Expect(err.Error()).To(ContainSubstring(dir))
				g.Expect(err.Error()).To(ContainSubstring(`"offending"`))
				g.Expect(err.Error()).To(ContainSubstring(tc.typeName))
				g.Expect(err.Error()).To(ContainSubstring("supported scalar types"))
				g.Expect(errors.Is(err, kustomize.ErrUnsupportedValue)).To(BeTrue())
			})
		}
	})

	t.Run("should not reject values for sources removed by selectors", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithSourceSelector(func(_ context.Context, _ kustomize.Source) (bool, error) {
				return false, nil
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Structured values are invalid for Kustomize sources, but the only
		// registered source is deselected, so it must be skipped without
		// constraining the render-time values (for example values aimed at
		// another renderer in the same engine pipeline).
		objects, err := renderer.Process(t.Context(), types.Values{"offending": map[string]any{"a": "1"}})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objects).To(BeEmpty())
	})

	t.Run("should reject non-finite floats", func(t *testing.T) {
		testCases := []struct {
			name  string
			value float64
		}{
			{"NaN", math.NaN()},
			{"positive infinity", math.Inf(1)},
			{"negative infinity", math.Inf(-1)},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				g := NewWithT(t)
				dir := setupBasicKustomization(t)

				renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
				g.Expect(err).ToNot(HaveOccurred())

				_, err = renderer.Process(t.Context(), types.Values{"ratio": tc.value})
				g.Expect(err).To(HaveOccurred())

				g.Expect(err.Error()).To(ContainSubstring(`"ratio"`))
				g.Expect(err.Error()).To(ContainSubstring("non-finite"))
				g.Expect(errors.Is(err, kustomize.ErrUnsupportedValue)).To(BeTrue())
			})
		}
	})

	t.Run("should reject non-finite float32 values", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		_, err = renderer.Process(t.Context(), types.Values{"ratio": float32(math.NaN())})
		g.Expect(err).To(HaveOccurred())

		g.Expect(err.Error()).To(ContainSubstring(`"ratio"`))
		g.Expect(err.Error()).To(ContainSubstring("non-finite"))
		g.Expect(errors.Is(err, kustomize.ErrUnsupportedValue)).To(BeTrue())
	})

	t.Run("should not format Stringer values or leak rejected contents", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		_, err = renderer.Process(t.Context(), types.Values{
			"secret": secretStringer{},
			"nested": map[string]any{"classified": "top-secret-content"},
		})
		g.Expect(err).To(HaveOccurred())

		// Rejected values are described by type only; neither the Stringer
		// output nor the nested map contents may reach the error message.
		g.Expect(err.Error()).ToNot(ContainSubstring("secret-stringer-output"))
		g.Expect(err.Error()).ToNot(ContainSubstring("top-secret-content"))
		g.Expect(errors.Is(err, kustomize.ErrUnsupportedValue)).To(BeTrue())
	})

	t.Run("should reject a self-referential map value instead of recursing", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		cyclic := map[string]any{}
		cyclic["self"] = cyclic

		// Before the fix this exhausted the goroutine stack inside the shared
		// clone helper; it must now fail with the key-specific validation
		// error, rejected by type before any recursive copy.
		_, err = renderer.Process(t.Context(), types.Values{"cyclic": cyclic})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring(`"cyclic"`))
		g.Expect(err.Error()).To(ContainSubstring("map[string]interface {}"))
		g.Expect(errors.Is(err, kustomize.ErrUnsupportedValue)).To(BeTrue())
	})

	t.Run("should reject a self-referential slice value instead of recursing", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New([]kustomize.Source{{Path: dir}})
		g.Expect(err).ToNot(HaveOccurred())

		cyclic := make([]any, 1)
		cyclic[0] = cyclic

		_, err = renderer.Process(t.Context(), types.Values{"cyclic": cyclic})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring(`"cyclic"`))
		g.Expect(err.Error()).To(ContainSubstring("[]interface {}"))
		g.Expect(errors.Is(err, kustomize.ErrUnsupportedValue)).To(BeTrue())
	})

	t.Run("should reject invalid values on cold and warm caches with a constant custom key", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupBasicKustomization(t)

		renderer, err := kustomize.New(
			[]kustomize.Source{{Path: dir}},
			kustomize.WithCache(cache.WithKeyFunc(constantKeyFunc)),
			kustomize.WithSourceAnnotations(true),
		)
		g.Expect(err).ToNot(HaveOccurred())

		invalid := types.Values{"nested": map[string]any{"a": "1"}}

		// Cold cache: the error fires before any build or cache lookup.
		_, err = renderer.Process(t.Context(), invalid)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, kustomize.ErrUnsupportedValue)).To(BeTrue())

		// Warm the cache with a valid render, then prove the entry is hot.
		live, err := renderer.Process(t.Context(), types.Values{"key": "value"})
		g.Expect(err).ToNot(HaveOccurred())
		for i := range live {
			g.Expect(live[i].GetAnnotations()).To(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginLive,
			))
		}

		cached, err := renderer.Process(t.Context(), types.Values{"key": "value"})
		g.Expect(err).ToNot(HaveOccurred())
		for i := range cached {
			g.Expect(cached[i].GetAnnotations()).To(HaveKeyWithValue(
				types.AnnotationRenderOrigin,
				types.RenderOriginCache,
			))
		}

		// Warm cache: the constant key cannot suppress the invalid-input
		// error, even though a matching entry is cached.
		_, err = renderer.Process(t.Context(), invalid)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, kustomize.ErrUnsupportedValue)).To(BeTrue())
	})
}

func TestEngineRenderValues(t *testing.T) {

	t.Run("should expose render-time scalars through the values ConfigMap", func(t *testing.T) {
		g := NewWithT(t)
		dir := setupValuesKustomization(t)

		e, err := kustomize.NewEngine(kustomize.Source{Path: dir})
		g.Expect(err).ToNot(HaveOccurred())

		objects, err := e.Render(t.Context(), render.WithValues(types.Values{
			"replicas": 3,
			"enabled":  true,
		}))
		g.Expect(err).ToNot(HaveOccurred())

		valuesConfigMap := findValuesConfigMap(objects)
		g.Expect(valuesConfigMap).ToNot(BeNil())

		g.Expect(valuesConfigMap.Object["data"]).To(Equal(map[string]any{
			"replicas": "3",
			"enabled":  "true",
		}))
	})
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
