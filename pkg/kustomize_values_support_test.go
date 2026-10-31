package kustomize_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// valuesKustomization consumes the virtual values.yaml ConfigMap as a
// resource, so rendered values are observable in the output objects.
const valuesKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- configmap.yaml
- values.yaml
`

// setupValuesKustomization creates a kustomization that renders the virtual
// values ConfigMap alongside a plain ConfigMap.
func setupValuesKustomization(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	writeFile(t, dir, "kustomization.yaml", valuesKustomization)
	writeFile(t, dir, "configmap.yaml", basicConfigMap)

	return dir
}

// findValuesConfigMap returns the rendered virtual values ConfigMap, or nil
// when the render did not produce one.
func findValuesConfigMap(objects []unstructured.Unstructured) *unstructured.Unstructured {
	for i := range objects {
		if objects[i].GetKind() == "ConfigMap" && objects[i].GetName() == "values" {
			return &objects[i]
		}
	}

	return nil
}
