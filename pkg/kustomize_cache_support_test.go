package kustomize_test

import (
	"context"

	"github.com/k8s-manifest-kit/engine/pkg/types"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// cacheIsolationLabel is the label used to identify which registered source
// produced an object when several sources render the same path.
const cacheIsolationLabel = "cache-isolation.k8s-manifest-kit/source"

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
