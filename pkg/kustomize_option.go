package kustomize

import (
	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util"
	"github.com/k8s-manifest-kit/pkg/util/cache"
	"sigs.k8s.io/kustomize/api/resmap"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RendererOption is a generic option for RendererOptions.
type RendererOption = util.Option[RendererOptions]

// RendererOptions is a struct-based option that can set multiple renderer options at once.
type RendererOptions struct {
	// Filters are renderer-specific filters applied during Process().
	Filters []types.Filter

	// Transformers are post-processing transformers applied after kustomize rendering.
	Transformers []types.Transformer

	// Plugins are kustomize-native transformer plugins applied during kustomize build.
	Plugins []resmap.Transformer

	// Cache is a custom cache implementation for render results.
	Cache cache.Interface[[]unstructured.Unstructured]

	// SourceAnnotations enables automatic addition of source tracking annotations.
	SourceAnnotations bool

	// LoadRestrictions sets renderer-wide default for load restrictions.
	// Individual Sources can override this via Source.LoadRestrictions.
	// Default: LoadRestrictionsRootOnly (security best practice).
	LoadRestrictions kustomizetypes.LoadRestrictions

	// CacheKeyFunc customizes how cache keys are generated from kustomization specifications.
	// If nil, DefaultCacheKey is used.
	CacheKeyFunc CacheKeyFunc
}

// ApplyTo applies the renderer options to the target configuration.
func (opts RendererOptions) ApplyTo(target *RendererOptions) {
	target.Filters = opts.Filters
	target.Transformers = opts.Transformers
	target.Plugins = opts.Plugins
	target.LoadRestrictions = opts.LoadRestrictions

	if opts.Cache != nil {
		target.Cache = opts.Cache
	}

	target.SourceAnnotations = opts.SourceAnnotations

	if opts.CacheKeyFunc != nil {
		target.CacheKeyFunc = opts.CacheKeyFunc
	}
}

// WithFilter adds a renderer-specific filter to this Kustomize renderer's processing chain.
// Renderer-specific filters are applied during Process(), before results are returned to the engine.
// For engine-level filtering applied to all renderers, use engine.WithFilter.
func WithFilter(f types.Filter) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Filters = append(opts.Filters, f)
	})
}

// WithTransformer adds a renderer-specific transformer to this Kustomize renderer's processing chain.
// Renderer-specific transformers are applied during Process(), before results are returned to the engine.
// For engine-level transformation applied to all renderers, use engine.WithTransformer.
func WithTransformer(t types.Transformer) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Transformers = append(opts.Transformers, t)
	})
}

// WithPlugin registers a plugin transformer (resmap.Transformer) for kustomize.
func WithPlugin(plugin resmap.Transformer) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Plugins = append(opts.Plugins, plugin)
	})
}

// WithCache enables render result caching with the specified options.
// If no options are provided, uses default TTL of 5 minutes.
// By default, caching is NOT enabled.
func WithCache(opts ...cache.Option) RendererOption {
	return util.FunctionalOption[RendererOptions](func(rendererOpts *RendererOptions) {
		rendererOpts.Cache = cache.NewRenderCache(opts...)
	})
}

// WithSourceAnnotations enables or disables automatic addition of source tracking annotations.
// When enabled, the renderer adds metadata annotations to track the source type and path.
// Annotations added: manifests.k8s-manifests-lib/source.type, source.path.
// Default: false (disabled).
func WithSourceAnnotations(enabled bool) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.SourceAnnotations = enabled
	})
}

// WithLoadRestrictions sets the renderer-wide default LoadRestrictions.
// Valid values: LoadRestrictionsRootOnly (default), LoadRestrictionsNone, LoadRestrictionsUnknown.
// Individual Sources can override this via Source.LoadRestrictions field.
//
// LoadRestrictionsRootOnly: Kustomization can only reference files within its own directory tree (secure).
// LoadRestrictionsNone: Kustomization can reference files anywhere on the filesystem (flexible but less secure).
func WithLoadRestrictions(restrictions kustomizetypes.LoadRestrictions) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.LoadRestrictions = restrictions
	})
}

// WithCacheKeyFunc sets a custom cache key generation function.
// Built-in options: DefaultCacheKey (default), FastCacheKey, PathOnlyCacheKey.
//
// Example:
//
//	kustomize.WithCacheKeyFunc(kustomize.FastCacheKey())
func WithCacheKeyFunc(fn CacheKeyFunc) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.CacheKeyFunc = fn
	})
}
