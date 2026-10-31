package kustomize_test

import (
	"os"
	"path/filepath"
	"testing"
)

const basicKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namePrefix: test-

resources:
- configmap.yaml
- pod.yaml
`

const basicConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: configmap
data:
  key: value
`

const basicPod = `
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
`

const baseKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- configmap.yaml
`

const baseConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  env: dev
`

const overlayKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

commonLabels:
  environment: production

resources:
- ../base

patches:
- patch: |-
    - op: replace
      path: /data/env
      value: prod
  target:
    kind: ConfigMap
`

const secondKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- service.yaml
`

const secondService = `
apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  ports:
  - port: 80
`

const labelsNamespaceKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: custom-namespace

commonLabels:
  app: myapp

resources:
- deployment.yaml
`

const labelsNamespaceDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: placeholder
  template:
    metadata:
      labels:
        app: placeholder
    spec:
      containers:
      - name: nginx
        image: nginx:latest
`

// Test constants for LoadRestrictions test.
const kustomizationWithParent = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../configmap.yaml
`

// Helper functions to set up test kustomizations

func setupBasicKustomization(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "kustomization.yaml", basicKustomization)
	writeFile(t, dir, "configmap.yaml", basicConfigMap)
	writeFile(t, dir, "pod.yaml", basicPod)

	return dir
}

func setupOverlayKustomization(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "base/kustomization.yaml", baseKustomization)
	writeFile(t, dir, "base/configmap.yaml", baseConfigMap)
	writeFile(t, dir, "overlay/kustomization.yaml", overlayKustomization)

	return dir
}

func setupSecondKustomization(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "kustomization.yaml", secondKustomization)
	writeFile(t, dir, "service.yaml", secondService)

	return dir
}

func setupKustomizationWithLabelsAndNamespace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "kustomization.yaml", labelsNamespaceKustomization)
	writeFile(t, dir, "deployment.yaml", labelsNamespaceDeployment)

	return dir
}

func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}

	err := os.WriteFile(path, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}
}
