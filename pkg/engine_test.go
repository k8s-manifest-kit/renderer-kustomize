package kustomize_test

import (
	"testing"

	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"

	. "github.com/onsi/gomega"
)

func TestNewEngine(t *testing.T) {

	t.Run("should create engine with Kustomize renderer", func(t *testing.T) {
		g := NewWithT(t)
		e, err := kustomize.NewEngine(kustomize.Source{
			Path: "../config/test/kustomizations/simple",
		})

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(e).ShouldNot(BeNil())
	})

	t.Run("should return error for invalid source", func(t *testing.T) {
		g := NewWithT(t)
		e, err := kustomize.NewEngine(kustomize.Source{
			Path: "", // Missing path
		})

		g.Expect(err).Should(HaveOccurred())
		g.Expect(e).Should(BeNil())
	})
}
