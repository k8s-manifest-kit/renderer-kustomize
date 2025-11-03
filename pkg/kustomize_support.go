package kustomize

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/k8s-manifest-kit/pkg/util"
	utilerrors "github.com/k8s-manifest-kit/pkg/util/errors"
	goyaml "gopkg.in/yaml.v3"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Values returns a Values function that always returns the provided static values.
// This is a convenience helper for the common case of non-dynamic values.
func Values(values map[string]string) func(context.Context) (map[string]string, error) {
	return func(_ context.Context) (map[string]string, error) {
		return values, nil
	}
}

var (
	//nolint:gochecknoglobals
	kustomizationFiles = []string{
		"kustomization.yaml",
		"kustomization.yml",
		"Kustomization",
	}

	// ErrNoKustomizationFile is returned when no kustomization file is found in a directory.
	ErrNoKustomizationFile = errors.New("no kustomization file found")
)

// sourceHolder wraps a Source with internal state for consistency with other renderers.
type sourceHolder struct {
	Source
}

// Validate checks if the Source configuration is valid.
func (h *sourceHolder) Validate() error {
	if len(strings.TrimSpace(h.Path)) == 0 {
		return utilerrors.ErrPathEmpty
	}

	return nil
}

func computeValues(ctx context.Context, input Source, renderTimeValues map[string]any) (map[string]string, error) {
	sourceValues := map[string]any{}

	if input.Values != nil {
		v, err := input.Values(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get values for kustomize path %q: %w", input.Path, err)
		}
		// Convert map[string]string to map[string]any for merging
		for k, v := range v {
			sourceValues[k] = v
		}
	}

	// Deep merge with render-time values taking precedence
	merged := util.DeepMerge(sourceValues, renderTimeValues)

	// Convert back to map[string]string
	result := make(map[string]string, len(merged))
	for k, v := range merged {
		result[k] = fmt.Sprintf("%v", v)
	}

	return result, nil
}

// createValuesConfigMapYAML creates the YAML content for a values ConfigMap.
// Does NOT write to filesystem - returns bytes for in-memory override.
func createValuesConfigMapYAML(values map[string]string) ([]byte, error) {
	configMap := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]string{
			"name": "values",
		},
		"data": values,
	}

	data, err := goyaml.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal values ConfigMap: %w", err)
	}

	return data, nil
}

func readKustomization(fs filesys.FileSystem, path string) (*kustomizetypes.Kustomization, string, error) {
	var kustName string
	var kustFile string

	for _, filename := range kustomizationFiles {
		candidate := filepath.Join(path, filename)
		if fs.Exists(candidate) {
			kustName = filename
			kustFile = candidate

			break
		}
	}

	if kustFile == "" {
		return nil, "", fmt.Errorf(
			"%w in %q (expected one of: %v)",
			ErrNoKustomizationFile,
			path,
			kustomizationFiles,
		)
	}

	content, err := fs.ReadFile(kustFile)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read kustomization from %s: %w", kustFile, err)
	}

	kust := &kustomizetypes.Kustomization{}
	if err := kust.Unmarshal(content); err != nil {
		return nil, "", fmt.Errorf(
			"failed to parse kustomization from %s (check YAML syntax): %w",
			kustFile,
			err,
		)
	}

	return kust, kustName, nil
}
