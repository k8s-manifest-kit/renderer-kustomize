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

	// Plugins are kustomize-native transformer plugins applied during kustomize build.
	Plugins []resmap.Transformer

	// CacheOptions holds cache configuration. nil = caching disabled.
	CacheOptions *cache.Options

	// SourceAnnotations enables automatic addition of source tracking annotations.
	SourceAnnotations bool

	// LoadRestrictions sets renderer-wide default for load restrictions.
	// Individual Sources can override this via Source.LoadRestrictions.
	// Default: LoadRestrictionsRootOnly (security best practice).
	LoadRestrictions kustomizetypes.LoadRestrictions

	// WarningHandler is called when kustomize deprecation warnings are detected.
	// If nil, warnings are logged to os.Stderr by default.
	WarningHandler WarningHandler

	// FileSystem specifies a custom filesystem to use for kustomize operations.
	// If nil, uses the OS filesystem (filesys.MakeFsOnDisk()).
	// This allows using embedded filesystems, in-memory filesystems, or custom implementations.
	FileSystem filesys.FileSystem
}

// ApplyTo applies the renderer options to the target configuration.
func (opts RendererOptions) ApplyTo(target *RendererOptions) {
	target.Filters = opts.Filters
	target.Transformers = opts.Transformers
	target.Plugins = opts.Plugins
	target.LoadRestrictions = opts.LoadRestrictions

	if opts.CacheOptions != nil {
		if target.CacheOptions == nil {
			target.CacheOptions = &cache.Options{}
		}
		opts.CacheOptions.ApplyTo(target.CacheOptions)
	}

	target.SourceAnnotations = opts.SourceAnnotations
	target.WarningHandler = opts.WarningHandler

	if opts.FileSystem != nil {
		target.FileSystem = opts.FileSystem
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
		if rendererOpts.CacheOptions == nil {
			rendererOpts.CacheOptions = &cache.Options{}
		}

		for _, opt := range opts {
			opt.ApplyTo(rendererOpts.CacheOptions)
		}
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

// WithWarningHandler sets a custom handler for kustomize deprecation warnings.
// The handler receives a list of warning messages and can choose to log them, fail, or ignore them.
// Use pre-built handlers like WarningLog(w), WarningFail(), or WarningIgnore(),
// or provide a custom function.
//
// Default: WarningLog(os.Stderr) if not set.
//
// Example:
//
//	kustomize.New(sources, kustomize.WithWarningHandler(kustomize.WarningFail()))
func WithWarningHandler(handler WarningHandler) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.WarningHandler = handler
	})
}

// WithFileSystem sets a custom filesystem for kustomize operations.
// This allows using embedded filesystems (via embed.FS), in-memory filesystems for testing,
// or any custom filesystem implementation.
//
// Default: Uses OS filesystem (filesys.MakeFsOnDisk()) if not set.
//
// Example with embedded filesystem:
//
//	//go:embed kustomizations/*
//	var embeddedFS embed.FS
//	fs, _ := fs.NewFromIOFS(embeddedFS, "kustomizations")
//	renderer := kustomize.New(sources, kustomize.WithFileSystem(fs))
//
// Example with in-memory filesystem:
//
//	memFs := fs.NewMemoryFs()
//	renderer := kustomize.New(sources, kustomize.WithFileSystem(memFs))
func WithFileSystem(fs filesys.FileSystem) RendererOption {
	return util.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.FileSystem = fs
	})
}
