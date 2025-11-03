package kustomize

import (
	"fmt"

	engine "github.com/k8s-manifest-kit/engine/pkg"
)

// NewEngine creates an Engine configured with a single Kustomize renderer.
// This is a convenience function for simple Kustomize-only rendering scenarios.
//
// Example:
//
//	e, _ := kustomize.NewEngine(
//	    kustomize.Source{
//	        Path: "/path/to/kustomization",
//	    },
//	    kustomize.WithLoadRestrictions(kustomizetypes.LoadRestrictionsRootOnly),
//	)
//	objects, _ := e.Render(ctx)
func NewEngine(source Source, opts ...RendererOption) (*engine.Engine, error) {
	renderer, err := New([]Source{source}, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create kustomize renderer: %w", err)
	}

	e, err := engine.New(engine.WithRenderer(renderer))
	if err != nil {
		return nil, fmt.Errorf("failed to create engine: %w", err)
	}

	return e, nil
}
