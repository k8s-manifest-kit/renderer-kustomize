package kustomize

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/k8s-manifest-kit/pkg/util/cache"
	utilerrors "github.com/k8s-manifest-kit/pkg/util/errors"
	utilmaps "github.com/k8s-manifest-kit/pkg/util/maps"
	goyaml "gopkg.in/yaml.v3"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	// ErrUnsupportedValue is returned when a render value is not a scalar the
	// Kustomize adapter can represent in its values ConfigMap. Source values
	// are map[string]string and always valid, so this error identifies a
	// render-time value.
	ErrUnsupportedValue = errors.New("unsupported render value")
)

// supportedValueTypes lists the value types the Kustomize adapter can
// represent in its values ConfigMap. It is quoted in validation errors so
// callers know which types to use instead.
const supportedValueTypes = "string, bool, integer, or finite float"

// sourceHolder wraps a Source with internal state for consistency with other renderers.
type sourceHolder struct {
	Source

	// cache isolates this source's rendered results from every other
	// registered source, so a source's load restrictions and post-rendering
	// pipeline can never be bypassed through another source's cached output,
	// regardless of the configured key function. It is nil when caching is
	// disabled.
	cache cache.Interface[[]unstructured.Unstructured]
}

// Validate checks if the Source configuration is valid.
func (h *sourceHolder) Validate() error {
	if len(strings.TrimSpace(h.Path)) == 0 {
		return utilerrors.ErrPathEmpty
	}

	return nil
}

// validateValues checks that every entry of the given values map is a scalar
// the Kustomize adapter can represent in its values ConfigMap. It is called at
// the renderer boundary, before any recursive copying of render-time values:
// the shared clone helpers have no cycle detection, so an unsupported
// composite value must be rejected here rather than traversed.
//
// Values are inspected by type only, so a self-referential map or slice is
// rejected by its type name instead of causing unbounded recursion. Keys are
// visited in sorted order to keep the reported error deterministic.
func validateValues(values map[string]any) error {
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if err := validateValue(key, values[key]); err != nil {
			return err
		}
	}

	return nil
}

// validateValue reports whether a single value is a scalar the Kustomize
// adapter can represent. Strings, booleans, and real integer, unsigned, and
// floating-point numbers are accepted; null, composites (maps and slices),
// and every other type are rejected by type name, without inspecting the
// value. Types implementing fmt.Stringer are not treated as numbers or
// strings.
func validateValue(key string, value any) error {
	switch v := value.(type) {
	case nil:
		return unsupportedValueError(key, "null")
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return nil
	case float32:
		return validateFiniteFloat(key, float64(v))
	case float64:
		return validateFiniteFloat(key, v)
	default:
		return unsupportedValueError(key, fmt.Sprintf("%T", value))
	}
}

// validateFiniteFloat rejects NaN and infinities. The values ConfigMap holds
// string data, and non-finite floats have no deterministic scalar
// representation, so they are rejected instead of being formatted lossily.
func validateFiniteFloat(key string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf(
			"%w for key %q: non-finite float values (NaN and infinity) are not supported",
			ErrUnsupportedValue,
			key,
		)
	}

	return nil
}

// unsupportedValueError describes a rejected value by key and type name only.
// The value itself is never included, so rejected input cannot leak into
// error messages or logs.
func unsupportedValueError(key string, valueType string) error {
	return fmt.Errorf(
		"%w for key %q: %s is not one of the supported scalar types (%s)",
		ErrUnsupportedValue,
		key,
		valueType,
		supportedValueTypes,
	)
}

func computeValues(ctx context.Context, input Source, renderTimeValues types.Values) (map[string]string, error) {
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

	merged := utilmaps.DeepMerge(sourceValues, map[string]any(renderTimeValues))

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
