package cache

import (
	utilcache "github.com/k8s-manifest-kit/pkg/util/cache"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// KustomizationSpec contains the data used to generate cache keys for rendered kustomizations.
type KustomizationSpec struct {
	Path   string
	Values map[string]string
}

// New creates a cache instance with Kustomize-specific default KeyFunc.
func New(opts *utilcache.Options) utilcache.Interface[[]unstructured.Unstructured] {
	if opts == nil {
		return nil
	}

	co := *opts

	// Inject default KeyFunc for Kustomize
	if co.KeyFunc == nil {
		co.KeyFunc = utilcache.DefaultKeyFunc
	}

	return utilcache.NewRenderCache(co)
}
