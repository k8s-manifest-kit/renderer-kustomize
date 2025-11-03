package kustomize

import (
	"github.com/k8s-manifest-kit/pkg/util/cache"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// KustomizationSpec contains the data used to generate cache keys for rendered kustomizations.
type KustomizationSpec struct {
	Path   string
	Values map[string]string
}

// newCache creates a cache instance with Kustomize-specific default KeyFunc.
func newCache(opts *cache.Options) cache.Interface[[]unstructured.Unstructured] {
	if opts == nil {
		return nil
	}

	co := *opts

	// Inject default KeyFunc for Kustomize
	if co.KeyFunc == nil {
		co.KeyFunc = cache.DefaultKeyFunc
	}

	return cache.NewRenderCache(co)
}
