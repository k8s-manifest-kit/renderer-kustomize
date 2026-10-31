package kustomize_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/xid"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"
)

func BenchmarkKustomizeRenderWithoutCache(b *testing.B) {
	dir := b.TempDir()

	writeFileB(b, dir, "kustomization.yaml", basicKustomization)
	writeFileB(b, dir, "configmap.yaml", basicConfigMap)
	writeFileB(b, dir, "pod.yaml", basicPod)

	renderer, err := kustomize.New([]kustomize.Source{
		{
			Path: dir,
			Values: kustomize.Values(map[string]string{
				"key1": "value1",
				"key2": "value2",
			}),
		},
	})
	if err != nil {
		b.Fatalf("failed to create renderer: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := renderer.Process(b.Context(), nil)
		if err != nil {
			b.Fatalf("failed to render: %v", err)
		}
	}
}

func BenchmarkKustomizeRenderWithCache(b *testing.B) {
	dir := b.TempDir()

	writeFileB(b, dir, "kustomization.yaml", basicKustomization)
	writeFileB(b, dir, "configmap.yaml", basicConfigMap)
	writeFileB(b, dir, "pod.yaml", basicPod)

	renderer, err := kustomize.New(
		[]kustomize.Source{
			{
				Path: dir,
				Values: kustomize.Values(map[string]string{
					"key1": "value1",
					"key2": "value2",
				}),
			},
		},
		kustomize.WithCache(),
	)
	if err != nil {
		b.Fatalf("failed to create renderer: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := renderer.Process(b.Context(), nil)
		if err != nil {
			b.Fatalf("failed to render: %v", err)
		}
	}
}

func BenchmarkKustomizeRenderCacheMiss(b *testing.B) {
	dir := b.TempDir()

	writeFileB(b, dir, "kustomization.yaml", basicKustomization)
	writeFileB(b, dir, "configmap.yaml", basicConfigMap)
	writeFileB(b, dir, "pod.yaml", basicPod)

	renderer, err := kustomize.New(
		[]kustomize.Source{
			{
				Path: dir,
				Values: func(_ context.Context) (map[string]string, error) {
					return map[string]string{
						"key": xid.New().String(),
					}, nil
				},
			},
		},
		kustomize.WithCache(),
	)
	if err != nil {
		b.Fatalf("failed to create renderer: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := renderer.Process(b.Context(), nil)
		if err != nil {
			b.Fatalf("failed to render: %v", err)
		}
	}
}

// Helper for benchmarks.
func writeFileB(b *testing.B, dir string, name string, content string) {
	b.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0600)
	if err != nil {
		b.Fatal(err)
	}
}
