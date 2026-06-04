package kustomize

import (
	"testing"

	kustomizetypes "sigs.k8s.io/kustomize/api/types"

	. "github.com/onsi/gomega"
)

func TestBuildKrustyOptions(t *testing.T) {

	t.Run("defaults match DisabledPluginConfig", func(t *testing.T) {
		g := NewWithT(t)

		opts := &RendererOptions{
			LoadRestrictions: kustomizetypes.LoadRestrictionsRootOnly,
		}

		result := buildKrustyOptions(opts, opts.LoadRestrictions)

		g.Expect(result.LoadRestrictions).To(Equal(kustomizetypes.LoadRestrictionsRootOnly))
		g.Expect(result.AddManagedbyLabel).To(BeFalse())
		g.Expect(result.PluginConfig).ToNot(BeNil())
		g.Expect(result.PluginConfig.BpLoadingOptions).To(Equal(kustomizetypes.BploUseStaticallyLinked))
		g.Expect(result.PluginConfig.PluginRestrictions).To(Equal(kustomizetypes.PluginRestrictionsBuiltinsOnly))
	})

	t.Run("AddManagedByLabel propagates", func(t *testing.T) {
		g := NewWithT(t)

		opts := &RendererOptions{
			AddManagedByLabel: true,
		}

		result := buildKrustyOptions(opts, kustomizetypes.LoadRestrictionsRootOnly)

		g.Expect(result.AddManagedbyLabel).To(BeTrue())
	})

	t.Run("BuiltinPluginLoadingOptions propagates", func(t *testing.T) {
		g := NewWithT(t)

		bpOpts := kustomizetypes.BploLoadFromFileSys
		opts := &RendererOptions{
			BuiltinPluginLoadingOptions: &bpOpts,
		}

		result := buildKrustyOptions(opts, kustomizetypes.LoadRestrictionsRootOnly)

		g.Expect(result.PluginConfig.BpLoadingOptions).To(Equal(kustomizetypes.BploLoadFromFileSys))
	})

	t.Run("FnPluginLoadingOptions propagates and opens restrictions", func(t *testing.T) {
		g := NewWithT(t)

		fnOpts := kustomizetypes.FnPluginLoadingOptions{
			EnableExec: true,
			Network:    true,
		}
		opts := &RendererOptions{
			FnPluginLoadingOptions: &fnOpts,
		}

		result := buildKrustyOptions(opts, kustomizetypes.LoadRestrictionsRootOnly)

		g.Expect(result.PluginConfig.FnpLoadingOptions.EnableExec).To(BeTrue())
		g.Expect(result.PluginConfig.FnpLoadingOptions.Network).To(BeTrue())
		g.Expect(result.PluginConfig.PluginRestrictions).To(Equal(kustomizetypes.PluginRestrictionsNone))
	})

	t.Run("per-source restrictions override renderer-wide", func(t *testing.T) {
		g := NewWithT(t)

		opts := &RendererOptions{
			LoadRestrictions: kustomizetypes.LoadRestrictionsRootOnly,
		}

		result := buildKrustyOptions(opts, kustomizetypes.LoadRestrictionsNone)

		g.Expect(result.LoadRestrictions).To(Equal(kustomizetypes.LoadRestrictionsNone))
	})

	t.Run("all options compose", func(t *testing.T) {
		g := NewWithT(t)

		bpOpts := kustomizetypes.BploLoadFromFileSys
		fnOpts := kustomizetypes.FnPluginLoadingOptions{
			EnableExec: true,
		}
		opts := &RendererOptions{
			AddManagedByLabel:           true,
			BuiltinPluginLoadingOptions: &bpOpts,
			FnPluginLoadingOptions:      &fnOpts,
		}

		result := buildKrustyOptions(opts, kustomizetypes.LoadRestrictionsNone)

		g.Expect(result.AddManagedbyLabel).To(BeTrue())
		g.Expect(result.LoadRestrictions).To(Equal(kustomizetypes.LoadRestrictionsNone))
		g.Expect(result.PluginConfig.BpLoadingOptions).To(Equal(kustomizetypes.BploLoadFromFileSys))
		g.Expect(result.PluginConfig.FnpLoadingOptions.EnableExec).To(BeTrue())
		g.Expect(result.PluginConfig.PluginRestrictions).To(Equal(kustomizetypes.PluginRestrictionsNone))
	})
}
