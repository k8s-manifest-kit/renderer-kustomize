package kustomize

import (
	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util"
	"github.com/k8s-manifest-kit/pkg/util/cache"
	"sigs.k8s.io/kustomize/api/resmap"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// RendererOption is a generic option for RendererOptions.
type RendererOption = util.Option[RendererOptions]

// RendererOptions is a struct-based option that can set multiple renderer options at once.
type RendererOptions struct {
	// Filters are renderer-specific filters applied during Process().
	Filters []types.Filter

	// Transformers are post-processing transformers applied after kustomize rendering.
	Transformers []types.Transformer

	// PostRenderers are renderer-specific post-renderers applied during Process().
	PostRenderers []types.PostRenderer

	// SourceSelectors are renderer-specific source selectors evaluated before rendering each source.
	SourceSelectors []types.SourceSelector

	// Plugins are kustomize-native transformer plugins applied during kustomize build.
	Plugins []resmap.Transformer

	// CacheOptions holds cache configuration. nil = caching disabled.
	CacheOptions *cache.Options

	// SourceAnnotations enables automatic addition of source tracking annotations.
	SourceAnnotations bool

	// ContentHash enables automatic addition of a SHA-256 content hash annotation.
	// Default: true (enabled).
	ContentHash bool

	// LoadRestrictions sets renderer-wide default for load restrictions.
	// Individual Sources can override this via Source.LoadRestrictions.
	// Default: LoadRestrictionsRootOnly (security best practice).
	LoadRestrictions kustomizetypes.LoadRestrictions

	// WarningHandler is called when kustomize deprecation warnings are detected.
	// If nil, warnings are logged to os.Stderr by default.
	WarningHandler WarningHandler

	// FileSystem specifies a custom filesystem to use for kustomize operations.
	// If nil, uses the OS filesystem (filesys.MakeFsOnDisk()).
	FileSystem filesys.FileSystem
}

// ApplyTo applies the renderer options to the target configuration.
func (opts RendererOptions) ApplyTo(target *RendererOptions) {
	target.Filters = opts.Filters
	target.Transformers = opts.Transformers
	target.PostRenderers = append(target.PostRenderers, opts.PostRenderers...)
	target.SourceSelectors = append(target.SourceSelectors, opts.SourceSelectors...)
	target.Plugins = opts.Plugins
	target.LoadRestrictions = opts.LoadRestrictions

	if opts.CacheOptions != nil {
		if target.CacheOptions == nil {
			target.CacheOptions = &cache.Options{}
		}
		opts.CacheOptions.ApplyTo(target.CacheOptions)
	}

	target.SourceAnnotations = opts.SourceAnnotations
	target.ContentHash = opts.ContentHash
	target.WarningHandler = opts.WarningHandler

	if opts.FileSystem != nil {
		target.FileSystem = opts.FileSystem
	}
}

// WithFilter adds a renderer-specific filter to this Kustomize renderer's processing chain.
func WithFilter(f types.Filter) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Filters = append(opts.Filters, f)
	})
}

// WithTransformer adds a renderer-specific transformer to this Kustomize renderer's processing chain.
func WithTransformer(t types.Transformer) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Transformers = append(opts.Transformers, t)
	})
}

// WithPostRenderer adds a renderer-specific post-renderer to this Kustomize renderer's processing chain.
func WithPostRenderer(p types.PostRenderer) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.PostRenderers = append(opts.PostRenderers, p)
	})
}

// WithSourceSelector adds a source selector to this Kustomize renderer.
// Use source.Selector[kustomize.Source] to build type-safe selectors.
func WithSourceSelector(s types.SourceSelector) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.SourceSelectors = append(opts.SourceSelectors, s)
	})
}

// WithPlugin registers a plugin transformer (resmap.Transformer) for kustomize.
func WithPlugin(plugin resmap.Transformer) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Plugins = append(opts.Plugins, plugin)
	})
}

// WithCache enables render result caching with the specified options.
func WithCache(opts ...cache.Option) RendererOption {
	return util.FunctionalOption[RendererOptions](func(rendererOpts *RendererOptions) {
		if rendererOpts.CacheOptions == nil {
			rendererOpts.CacheOptions = &cache.Options{}
		}

		for _, opt := range opts {
			opt.ApplyTo(rendererOpts.CacheOptions)
		}
	})
}

// WithSourceAnnotations enables or disables automatic addition of source tracking annotations.
func WithSourceAnnotations(enabled bool) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.SourceAnnotations = enabled
	})
}

// WithContentHash enables or disables automatic addition of a SHA-256 content hash annotation.
func WithContentHash(enabled bool) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.ContentHash = enabled
	})
}

// WithLoadRestrictions sets the renderer-wide default LoadRestrictions.
func WithLoadRestrictions(restrictions kustomizetypes.LoadRestrictions) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.LoadRestrictions = restrictions
	})
}

// WithWarningHandler sets a custom handler for kustomize deprecation warnings.
func WithWarningHandler(handler WarningHandler) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.WarningHandler = handler
	})
}

// WithFileSystem sets a custom filesystem for kustomize operations.
func WithFileSystem(fs filesys.FileSystem) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.FileSystem = fs
	})
}
