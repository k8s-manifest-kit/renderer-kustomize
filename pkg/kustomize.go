package kustomize

import (
	"context"
	"fmt"

	"github.com/k8s-manifest-kit/engine/pkg/pipeline"
	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"
	"sigs.k8s.io/kustomize/api/resmap"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-manifest-kit/renderer-kustomize/pkg/util/fs"
)

const rendererType = "kustomize"

// Source represents the input for a Kustomize rendering operation.
type Source struct {
	// Path specifies the directory containing kustomization.yaml.
	// Must be a valid filesystem path to a kustomization root.
	Path string

	// Values provides dynamic key-value data written as a ConfigMap.
	// Function is called during rendering to obtain dynamic values.
	// The values are written to a ConfigMap file at Path/values.yaml.
	//
	// IMPORTANT: Values are NOT applied automatically to resources.
	// The kustomization must explicitly use this ConfigMap via:
	// - replacements: to substitute values in resources
	// - configMapGenerator: if integrating with generated configs
	// - patches: to modify resources based on values
	//
	// If Path/values.yaml already exists, rendering will fail with an error
	// to prevent accidental overwrites.
	Values func(context.Context) (map[string]string, error)

	// LoadRestrictions specifies restrictions on what can be referenced.
	// If LoadRestrictionsUnknown (zero value), uses the renderer-wide default.
	// Set to LoadRestrictionsRootOnly or LoadRestrictionsNone to override.
	LoadRestrictions kustomizetypes.LoadRestrictions

	// PostRenderers are source-specific post-renderers applied to this source's output
	// before combining with other sources.
	PostRenderers []types.PostRenderer
}

// Renderer is a renderer that uses kustomize to render resources.
type Renderer struct {
	inputs []*sourceHolder
	fs     filesys.FileSystem
	engine *Engine
	opts   *RendererOptions
	cache  cache.Interface[[]unstructured.Unstructured]
}

// New creates a new kustomize renderer.
func New(inputs []Source, opts ...RendererOption) (*Renderer, error) {
	// Initialize renderer options
	rendererOpts := RendererOptions{
		Filters:          make([]types.Filter, 0),
		Transformers:     make([]types.Transformer, 0),
		Plugins:          make([]resmap.Transformer, 0),
		LoadRestrictions: kustomizetypes.LoadRestrictionsRootOnly,
		ContentHash:      true,
	}

	// Apply all options to RendererOptions
	for _, opt := range opts {
		opt.ApplyTo(&rendererOpts)
	}

	// Wrap sources in holders and validate
	holders := make([]*sourceHolder, len(inputs))
	for i := range inputs {
		holders[i] = &sourceHolder{
			Source: inputs[i],
		}
		if err := holders[i].Validate(); err != nil {
			return nil, err
		}
	}

	// Use custom filesystem if provided, otherwise default to OS filesystem
	fsys := rendererOpts.FileSystem
	if fsys == nil {
		fsys = fs.NewFsOnDisk()
	}

	r := &Renderer{
		inputs: holders,
		fs:     fsys,
		engine: newKustomizeEngine(fsys, &rendererOpts),
		opts:   &rendererOpts,
		cache:  newCache(rendererOpts.CacheOptions),
	}

	return r, nil
}

// Name returns the renderer type identifier.
func (r *Renderer) Name() string {
	return rendererType
}

// Process implements types.Renderer by rendering the kustomize resources and applying filters and transformers.
func (r *Renderer) Process(ctx context.Context, renderTimeValues types.Values) ([]unstructured.Unstructured, error) {
	allObjects := make([]unstructured.Unstructured, 0)

	for _, holder := range r.inputs {
		selected, err := pipeline.ApplySourceSelectors(ctx, holder.Source, r.opts.SourceSelectors)
		if err != nil {
			return nil, fmt.Errorf("source selector error for kustomize path %s: %w", holder.Path, err)
		}

		if !selected {
			continue
		}

		sValues := renderTimeValues.DeepClone()

		objects, err := r.renderSingle(ctx, holder, sValues)
		if err != nil {
			return nil, fmt.Errorf("error rendering kustomize path %s: %w", holder.Path, err)
		}

		objects, err = pipeline.ApplyPostRenderers(ctx, objects, holder.PostRenderers)
		if err != nil {
			return nil, fmt.Errorf("source post-renderer error for kustomize path %s: %w", holder.Path, err)
		}

		allObjects = append(allObjects, objects...)
	}

	chain := types.BuildPostRendererChain(r.opts.Filters, r.opts.Transformers, r.opts.PostRenderers)

	result, err := pipeline.ApplyPostRenderers(ctx, allObjects, chain)
	if err != nil {
		return nil, fmt.Errorf("renderer post-renderer error: %w", err)
	}

	return result, nil
}

// renderSingle performs the rendering for a single kustomize path.
func (r *Renderer) renderSingle(
	ctx context.Context,
	holder *sourceHolder,
	renderTimeValues types.Values,
) ([]unstructured.Unstructured, error) {
	// Get values dynamically (includes render-time values)
	values, err := computeValues(ctx, holder.Source, renderTimeValues)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get values for path %q: %w",
			holder.Path,
			err,
		)
	}

	spec := KustomizationSpec{
		Path:   holder.Path,
		Values: values,
	}

	// Check cache (if enabled)
	if r.cache != nil {
		// ensure objects are evicted
		r.cache.Sync()

		if cached, found := r.cache.Get(spec); found {
			return cached, nil
		}
	}

	// No filesystem writes needed - values passed to engine
	result, err := r.engine.Run(holder.Source, values)
	if err != nil {
		return nil, fmt.Errorf("failed to run kustomize for path %q: %w", holder.Path, err)
	}

	// Cache result (if enabled)
	if r.cache != nil {
		r.cache.Set(spec, result)
	}

	return result, nil
}
