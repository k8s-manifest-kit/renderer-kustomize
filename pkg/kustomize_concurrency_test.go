package kustomize_test

import (
	"context"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

// concurrentRenders is the number of overlapping Process calls issued against
// one shared renderer during each concurrent phase.
const concurrentRenders = 8

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

// runValueIsolationScenario exercises one shared renderer with barrier-started
// overlapping renders that each request distinct scalar values: a cold phase
// where every worker must observe exactly its requested values ConfigMap
// data, a nested mutation of one cold result after the workers join, and a
// warm phase whose results must stay independent of that mutation.
// warmOrigin is the render origin every warm render must report: cache hits
// when caching is enabled, live builds otherwise.
func runValueIsolationScenario(
	ctx context.Context,
	g *WithT,
	renderer *kustomize.Renderer,
	dir string,
	warmOrigin string,
) {
	// Cold phase: every worker's distinct scalar values must reach its own
	// invocation-local values.yaml overlay, never another call's overlay.
	coldResults, coldErrs := renderConcurrently(ctx, renderer, distinctWorkerValues)
	for id := range coldResults {
		g.Expect(coldErrs[id]).ToNot(HaveOccurred())
		expectWorkerObjects(g, coldResults[id], id, types.RenderOriginLive)
	}

	// The union overlay is invocation-local: values.yaml never leaks into the
	// immutable on-disk base filesystem.
	g.Expect(filepath.Join(dir, "values.yaml")).ToNot(BeAnExistingFile())

	// Mutate one worker's nested data after the workers joined: neither
	// another worker's cold result...
	mutated := findValuesConfigMap(coldResults[0])
	g.Expect(mutated).ToNot(BeNil())
	mutated.Object["data"] = map[string]any{"mutated": "true"}

	expectWorkerObjects(g, coldResults[1], 1, types.RenderOriginLive)

	// ...nor any later warm result may observe the mutation.
	warmResults, warmErrs := renderConcurrently(ctx, renderer, distinctWorkerValues)
	for id := range warmResults {
		g.Expect(warmErrs[id]).ToNot(HaveOccurred())
		expectWorkerObjects(g, warmResults[id], id, warmOrigin)
	}
}

// runSourceIsolationScenario exercises one shared renderer with two same-path
// sources behind a degenerate constant cache key. Workers request identical
// values, so within one source the constant key may legitimately serve
// another worker's freshly cached result, but the per-source caches must keep
// the sources' outputs separated and independently owned.
func runSourceIsolationScenario(
	ctx context.Context,
	g *WithT,
	renderer *kustomize.Renderer,
	dir string,
) {
	sharedValues := types.Values{"worker": "shared", "replicas": 1}
	shared := func(int) types.Values { return sharedValues }

	// Cold phase: caches start empty, so at least one worker must build each
	// source live even though other racing workers may already hit the
	// freshly populated per-source entries.
	coldResults, coldErrs := renderConcurrently(ctx, renderer, shared)
	for id := range coldResults {
		g.Expect(coldErrs[id]).ToNot(HaveOccurred())
		expectSourceIsolatedObjects(g, coldResults[id], types.RenderOriginLive, types.RenderOriginCache)
	}
	expectLiveLoadPerSource(g, coldResults)

	g.Expect(filepath.Join(dir, "values.yaml")).ToNot(BeAnExistingFile())

	mutated := findValuesConfigMap(coldResults[0])
	g.Expect(mutated).ToNot(BeNil())
	mutated.Object["data"] = map[string]any{"mutated": "true"}

	// Warm phase: every render is served from its own source's cache, the
	// mutation is invisible, and no source ever returns the other's output.
	warmResults, warmErrs := renderConcurrently(ctx, renderer, shared)
	for id := range warmResults {
		g.Expect(warmErrs[id]).ToNot(HaveOccurred())
		expectSourceIsolatedObjects(g, warmResults[id], types.RenderOriginCache)
	}
}

// distinctWorkerValues returns the scalar values one worker requests: a
// string and an integer both derived from the worker id, so every worker's
// requested values ConfigMap data is unique.
func distinctWorkerValues(id int) types.Values {
	return types.Values{
		"worker":   strconv.Itoa(id),
		"replicas": id,
	}
}

// renderConcurrently issues concurrentRenders overlapping Process calls
// against the shared renderer, each requesting the values its worker id maps
// to, started together by a barrier. Workers only write to their own slots;
// all assertions happen in the test goroutine after the workers join.
func renderConcurrently(
	ctx context.Context,
	renderer *kustomize.Renderer,
	valuesFor func(id int) types.Values,
) ([][]unstructured.Unstructured, []error) {
	results := make([][]unstructured.Unstructured, concurrentRenders)
	errs := make([]error, concurrentRenders)

	start := make(chan struct{})
	var done sync.WaitGroup

	for id := range concurrentRenders {
		done.Go(func() {
			<-start

			objects, err := renderer.Process(ctx, valuesFor(id))
			results[id] = objects
			errs[id] = err
		})
	}

	close(start)
	done.Wait()

	return results, errs
}

// expectWorkerObjects asserts that one render returned exactly the plain
// ConfigMap plus the virtual values ConfigMap, whose data carries exactly the
// given worker's requested scalar values, and an allowed render origin.
func expectWorkerObjects(
	g *WithT,
	objects []unstructured.Unstructured,
	id int,
	allowedOrigins ...string,
) {
	g.Expect(objects).To(HaveLen(2))

	names := make([]string, 0, len(objects))
	for i := range objects {
		names = append(names, objects[i].GetName())

		annotations := objects[i].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
		g.Expect(allowedOrigins).To(ContainElement(annotations[types.AnnotationRenderOrigin]))
	}

	g.Expect(names).To(ConsistOf("configmap", "values"))

	valuesConfigMap := findValuesConfigMap(objects)
	g.Expect(valuesConfigMap).ToNot(BeNil(), "values ConfigMap should be rendered")
	g.Expect(valuesConfigMap.Object["data"]).To(Equal(map[string]any{
		"worker":   strconv.Itoa(id),
		"replicas": strconv.Itoa(id),
	}))
}

// expectSourceIsolatedObjects asserts that one render returned two objects
// per registered source — the plain ConfigMap and the values ConfigMap —
// where every object carries exactly its own source's post-renderer label,
// the shared requested values, and an allowed render origin.
func expectSourceIsolatedObjects(
	g *WithT,
	objects []unstructured.Unstructured,
	allowedOrigins ...string,
) {
	g.Expect(objects).To(HaveLen(4))

	sources := map[string]int{}
	for i := range objects {
		labels := objects[i].GetLabels()
		source := labels[cacheIsolationLabel]
		sources[source]++

		// Exactly one source label per object: cached outputs must never
		// cross sources, even behind a constant custom key.
		g.Expect(labels).To(HaveLen(1))

		annotations := objects[i].GetAnnotations()
		g.Expect(annotations).To(HaveKeyWithValue(types.AnnotationSourceType, "kustomize"))
		g.Expect(allowedOrigins).To(ContainElement(annotations[types.AnnotationRenderOrigin]))

		if objects[i].GetName() == "values" {
			g.Expect(objects[i].Object["data"]).To(Equal(map[string]any{
				"worker":   "shared",
				"replicas": "1",
			}))
		}
	}

	g.Expect(sources).To(Equal(map[string]int{"alpha": 2, "beta": 2}))
}

// expectLiveLoadPerSource asserts that the cold phase exercised the live path
// for both sources: caches start empty, so at least one worker must have
// built each source live even though other racing workers may already have
// hit the freshly populated per-source entries.
func expectLiveLoadPerSource(g *WithT, results [][]unstructured.Unstructured) {
	liveSources := make(map[string]bool)

	for _, objects := range results {
		for i := range objects {
			annotations := objects[i].GetAnnotations()
			if annotations[types.AnnotationRenderOrigin] == types.RenderOriginLive {
				liveSources[objects[i].GetLabels()[cacheIsolationLabel]] = true
			}
		}
	}

	g.Expect(liveSources).To(HaveKey("alpha"))
	g.Expect(liveSources).To(HaveKey("beta"))
}
