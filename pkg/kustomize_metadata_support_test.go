package kustomize_test

// Test constants for nested resources test.
const nestedResourcesKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- resources/configmap.yaml
- resources/configs/secret.yaml
`

const nestedConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: nested-config
data:
  location: nested
`

//nolint:gosec // synthetic test Secret fixture, not a real credential
const nestedSecret = `
apiVersion: v1
kind: Secret
metadata:
  name: nested-secret
type: Opaque
stringData:
  key: value
`

// Test constants for base/overlay annotations test.
const annotationsBaseKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- configmap.yaml
- deployment.yaml
`

const annotationsBaseConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: base-config
data:
  env: base
`

const annotationsBaseDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: app
        image: nginx:latest
`

const annotationsOverlayKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../base

commonLabels:
  overlay: "true"
`

// Test constants for multiple components test.
const componentsMainKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- components/frontend
- components/backend
`

const componentsFrontendKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- deployment.yaml
`

const componentsFrontendDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: frontend
  template:
    metadata:
      labels:
        app: frontend
    spec:
      containers:
      - name: frontend
        image: nginx:latest
`

const componentsBackendKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- deployment.yaml
- service.yaml
`

const componentsBackendDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
    spec:
      containers:
      - name: backend
        image: nginx:latest
`

const componentsBackendService = `
apiVersion: v1
kind: Service
metadata:
  name: backend
spec:
  selector:
    app: backend
  ports:
  - port: 80
`
