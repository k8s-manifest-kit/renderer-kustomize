package kustomize

import (
	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"
	"github.com/k8s-manifest-kit/pkg/util/option"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// RendererOption is a generic option for RendererOptions.
type RendererOption = option.Option[RendererOptions]

// RendererOptions is a struct-based option that can set multiple renderer options at once.
//
// Options apply left to right: each option sees the state left by the ones
// before it. When RendererOptions is applied through ApplyTo, fields divide
// into replacing and preserving groups:
//
//   - Filters, Transformers, PostRenderers, SourceSelectors, and Plugins
//     replace the target's slices, including when nil — a struct clears
//     hooks, selectors, and plugin transformers that earlier options added.
//   - LoadRestrictions replaces the target's value including with its zero
//     value LoadRestrictionsUnknown — see the field comment for why an
//     explicit value matters.
//   - SourceAnnotations, ContentHash, and AddManagedByLabel replace the
//     target's booleans, including when false.
//   - WarningHandler replaces the target's value including when nil, which
//     reverts warnings to kustomize's own stderr output.
//   - A nil CacheOptions preserves the target's cache configuration; a
//     non-nil CacheOptions applies its own field rules to the target's cache
//     options, creating them if needed.
//   - Nil FileSystem, FnPluginLoadingOptions, and BuiltinPluginLoadingOptions
//     preserve the target's configuration; non-nil pointers replace it.
//
// Functional options change only their own field or append to a single
// slice, so they compose with anything applied before or after. Note the
// difference from a fresh constructor: New starts from defaults —
// LoadRestrictionsRootOnly, content hashing enabled, caching disabled, the
// OS on-disk filesystem, and no plugin-loading configuration — while
// applying a RendererOptions struct replaces the fields above outright. A
// caller who wants content hashing or root-only loading after a struct must
// set ContentHash: true or LoadRestrictions:
// kustomizetypes.LoadRestrictionsRootOnly in the struct (or follow it with
// the corresponding functional option).
type RendererOptions struct {
	// Filters are renderer-specific filters applied during Process().
	Filters []types.Filter

	// Transformers are post-processing transformers applied after kustomize rendering.
	Transformers []types.Transformer

	// PostRenderers are renderer-specific post-renderers applied during Process().
	PostRenderers []types.PostRenderer

	// SourceSelectors are renderer-specific source selectors evaluated before rendering each source.
	SourceSelectors []SourceSelector

	// Plugins are kustomize-native transformer plugins applied during kustomize build.
	// Applying a RendererOptions struct replaces this slice, including when nil.
	Plugins []resmap.Transformer

	// CacheOptions holds cache configuration. nil = caching disabled.
	// Applying a RendererOptions struct with a nil CacheOptions preserves
	// the previously configured cache (including disabled); a non-nil value
	// applies its field rules on top of the previous configuration.
	CacheOptions *cache.Options

	// SourceAnnotations enables automatic addition of source tracking annotations.
	// Applying a RendererOptions struct replaces this field, including with false.
	SourceAnnotations bool

	// ContentHash enables automatic addition of a SHA-256 content hash annotation.
	// Default: true (enabled) in a fresh constructor; applying a
	// RendererOptions struct replaces this field, including with false, so a
	// struct that wants hashing must set it to true explicitly.
	ContentHash bool

	// LoadRestrictions sets renderer-wide default for load restrictions.
	// Individual Sources can override this via Source.LoadRestrictions.
	// Default: LoadRestrictionsRootOnly (security best practice).
	//
	// Applying a RendererOptions struct replaces this field including with
	// its zero value LoadRestrictionsUnknown, discarding the constructor's
	// RootOnly default. With the renderer-wide value Unknown, only a
	// source-level restriction governs; a source that also leaves it
	// Unknown is built with kustomize's unrestricted loader, because
	// kustomize's Run treats every value other than RootOnly as
	// RestrictionNone, so files outside the kustomization root may be
	// loaded. A struct that wants root-only loading must set it explicitly.
	LoadRestrictions kustomizetypes.LoadRestrictions

	// WarningHandler is called when kustomize deprecation warnings are detected.
	// If nil, kustomize's own stderr output is the only warning signal.
	// Use WarningFail() to abort rendering on warnings, WarningLog(w) to
	// redirect warnings to a custom writer, or WarningIgnore() as a no-op.
	//
	// Applying a RendererOptions struct replaces this field including when
	// nil, which reverts warnings to kustomize's stderr output.
	WarningHandler WarningHandler

	// FileSystem specifies a custom filesystem to use for kustomize operations.
	// If nil, uses the OS filesystem (filesys.MakeFsOnDisk()).
	//
	// Applying a RendererOptions struct with a nil FileSystem preserves the
	// previously configured filesystem; a non-nil value replaces it.
	FileSystem filesys.FileSystem

	// FnPluginLoadingOptions configures KRM function and exec plugin behavior.
	// When set, PluginRestrictions is automatically opened to PluginRestrictionsNone
	// since function-based plugins require it.
	// Note: kustomize's plugin loader always reads plugin configs from the real
	// disk, regardless of the FileSystem passed via WithFileSystem.
	// Default: nil (function-based plugins disabled).
	//
	// Applying a RendererOptions struct with a nil value preserves the
	// previous configuration; a non-nil pointer replaces it.
	FnPluginLoadingOptions *kustomizetypes.FnPluginLoadingOptions

	// BuiltinPluginLoadingOptions controls how builtin plugins are loaded.
	// Default: BploUseStaticallyLinked.
	//
	// Applying a RendererOptions struct with a nil value preserves the
	// previous configuration; a non-nil pointer replaces it.
	BuiltinPluginLoadingOptions *kustomizetypes.BuiltinPluginLoadingOptions

	// AddManagedByLabel adds app.kubernetes.io/managed-by: kustomize-<version> to all resources.
	// Default: false.
	//
	// Applying a RendererOptions struct replaces this field, including with false.
	AddManagedByLabel bool
}

// ApplyTo applies the renderer options to the target configuration.
//
// Application follows the composition rules documented on RendererOptions:
// hooks, selectors, plugins, load restrictions, the warning handler, and the
// booleans replace their target fields (including with nil, the zero value,
// and false), while nil CacheOptions, FileSystem, and plugin-loading
// pointers preserve the target's configuration and non-nil values replace
// or apply their field rules on top.
func (opts RendererOptions) ApplyTo(target *RendererOptions) {
	target.Filters = opts.Filters
	target.Transformers = opts.Transformers
	target.PostRenderers = opts.PostRenderers
	target.SourceSelectors = opts.SourceSelectors
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

	if opts.FnPluginLoadingOptions != nil {
		target.FnPluginLoadingOptions = opts.FnPluginLoadingOptions
	}

	if opts.BuiltinPluginLoadingOptions != nil {
		target.BuiltinPluginLoadingOptions = opts.BuiltinPluginLoadingOptions
	}

	target.AddManagedByLabel = opts.AddManagedByLabel
}

// WithFilter adds a renderer-specific filter to this Kustomize renderer's processing chain.
func WithFilter(f types.Filter) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Filters = append(opts.Filters, f)
	})
}

// WithTransformer adds a renderer-specific transformer to this Kustomize renderer's processing chain.
func WithTransformer(t types.Transformer) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Transformers = append(opts.Transformers, t)
	})
}

// WithPostRenderer adds a renderer-specific post-renderer to this Kustomize renderer's processing chain.
func WithPostRenderer(p types.PostRenderer) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.PostRenderers = append(opts.PostRenderers, p)
	})
}

// WithSourceSelector adds a source selector to this Kustomize renderer.
// Source selectors are evaluated before rendering each source. If any selector
// returns false, the source is skipped entirely.
func WithSourceSelector(s SourceSelector) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.SourceSelectors = append(opts.SourceSelectors, s)
	})
}

// WithPlugin registers a plugin transformer (resmap.Transformer) for kustomize.
func WithPlugin(plugin resmap.Transformer) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.Plugins = append(opts.Plugins, plugin)
	})
}

// WithCache enables render result caching with the specified options.
func WithCache(opts ...cache.Option) RendererOption {
	return option.FunctionalOption[RendererOptions](func(rendererOpts *RendererOptions) {
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
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.SourceAnnotations = enabled
	})
}

// WithContentHash enables or disables automatic addition of a SHA-256 content hash annotation.
func WithContentHash(enabled bool) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.ContentHash = enabled
	})
}

// WithLoadRestrictions sets the renderer-wide default LoadRestrictions.
func WithLoadRestrictions(restrictions kustomizetypes.LoadRestrictions) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.LoadRestrictions = restrictions
	})
}

// WithWarningHandler sets a custom handler for kustomize deprecation warnings.
func WithWarningHandler(handler WarningHandler) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.WarningHandler = handler
	})
}

// WithFileSystem sets a custom filesystem for kustomize operations.
func WithFileSystem(fs filesys.FileSystem) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.FileSystem = fs
	})
}

// WithFnPluginLoadingOptions configures KRM function and exec plugin behavior.
// Setting this automatically opens PluginRestrictions to PluginRestrictionsNone.
func WithFnPluginLoadingOptions(fnOpts kustomizetypes.FnPluginLoadingOptions) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.FnPluginLoadingOptions = &fnOpts
	})
}

// WithBuiltinPluginLoadingOptions controls how builtin plugins are loaded
// (statically linked vs. from filesystem).
func WithBuiltinPluginLoadingOptions(bpOpts kustomizetypes.BuiltinPluginLoadingOptions) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.BuiltinPluginLoadingOptions = &bpOpts
	})
}

// WithManagedByLabel enables or disables adding the app.kubernetes.io/managed-by label.
func WithManagedByLabel(enabled bool) RendererOption {
	return option.FunctionalOption[RendererOptions](func(opts *RendererOptions) {
		opts.AddManagedByLabel = enabled
	})
}

// buildKrustyOptions constructs krusty.Options from RendererOptions and per-source load restrictions.
func buildKrustyOptions(rendererOpts *RendererOptions, restrictions kustomizetypes.LoadRestrictions) *krusty.Options {
	opts := &krusty.Options{
		LoadRestrictions:  restrictions,
		AddManagedbyLabel: rendererOpts.AddManagedByLabel,
		PluginConfig:      kustomizetypes.DisabledPluginConfig(),
	}

	if rendererOpts.BuiltinPluginLoadingOptions != nil {
		opts.PluginConfig.BpLoadingOptions = *rendererOpts.BuiltinPluginLoadingOptions
	}

	if rendererOpts.FnPluginLoadingOptions != nil {
		opts.PluginConfig.FnpLoadingOptions = *rendererOpts.FnPluginLoadingOptions
		opts.PluginConfig.PluginRestrictions = kustomizetypes.PluginRestrictionsNone
	}

	return opts
}
