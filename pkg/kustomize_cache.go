package kustomize

import (
	"k8s.io/apimachinery/pkg/util/dump"
)

// KustomizationSpec contains the data used to generate cache keys for rendered kustomizations.
type KustomizationSpec struct {
	Path   string
	Values map[string]string
}

// CacheKeyFunc generates a cache key from kustomization specification.
type CacheKeyFunc func(KustomizationSpec) string

// DefaultCacheKey returns a CacheKeyFunc that uses reflection-based hashing of all
// kustomization specification fields. This is the safest option but may be slower for
// large value structures.
//
// Security Considerations:
// Cache keys are generated from kustomization values which may contain sensitive data such as
// passwords, API tokens, or other secrets. The resulting hash is deterministic and could
// potentially leak information if logged or exposed. For kustomizations with sensitive values:
//   - Avoid logging cache keys in production environments
//   - Consider using FastCacheKey() or PathOnlyCacheKey() which ignore values
//   - Implement a custom CacheKeyFunc that excludes sensitive fields
//
// Example with sensitive data:
//
//	// If your values contain secrets, consider alternative cache key strategies
//	renderer := kustomize.New(sources, kustomize.WithCacheKeyFunc(kustomize.FastCacheKey()))
func DefaultCacheKey() CacheKeyFunc {
	return func(spec KustomizationSpec) string {
		return dump.ForHash(spec)
	}
}

// FastCacheKey returns a CacheKeyFunc that generates keys based only on kustomization path,
// ignoring values. Use this when values don't affect the rendered output, when performance
// is critical, or when values may contain sensitive data that should not be included in
// cache keys.
func FastCacheKey() CacheKeyFunc {
	return func(spec KustomizationSpec) string {
		return spec.Path
	}
}

// PathOnlyCacheKey returns a CacheKeyFunc that generates keys based only on the
// kustomization path. This is an alias for FastCacheKey provided for clarity when
// the intent is to cache purely by path regardless of values.
func PathOnlyCacheKey() CacheKeyFunc {
	return func(spec KustomizationSpec) string {
		return spec.Path
	}
}
