package output

import (
	"sort"
	"testing"
)

func TestRegistryRegisterAndCreate(t *testing.T) {
	reg := NewRegistry()
	reg.Register(DeliveryMSE, func(cfg PluginConfig) (OutputPlugin, error) {
		return newMockPlugin(DeliveryMSE), nil
	})

	p, err := reg.Create(DeliveryMSE, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Mode() != DeliveryMSE {
		t.Fatalf("expected mode %s, got %s", DeliveryMSE, p.Mode())
	}
}

func TestRegistryUnknownModeReturnsError(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Create(DeliveryHLS, PluginConfig{})
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestRegistryModes(t *testing.T) {
	reg := NewRegistry()
	reg.Register(DeliveryMSE, func(cfg PluginConfig) (OutputPlugin, error) {
		return newMockPlugin(DeliveryMSE), nil
	})
	reg.Register(DeliveryHLS, func(cfg PluginConfig) (OutputPlugin, error) {
		return newMockPlugin(DeliveryHLS), nil
	})

	modes := reg.Modes()
	sort.Slice(modes, func(i, j int) bool { return modes[i] < modes[j] })

	if len(modes) != 2 {
		t.Fatalf("expected 2 modes, got %d", len(modes))
	}
	if modes[0] != DeliveryHLS {
		t.Errorf("expected hls, got %s", modes[0])
	}
	if modes[1] != DeliveryMSE {
		t.Errorf("expected mse, got %s", modes[1])
	}
}

func TestRegistryOverwriteFactory(t *testing.T) {
	reg := NewRegistry()

	reg.Register(DeliveryMSE, func(cfg PluginConfig) (OutputPlugin, error) {
		p := newMockPlugin(DeliveryMSE)
		p.healthy = false
		return p, nil
	})

	reg.Register(DeliveryMSE, func(cfg PluginConfig) (OutputPlugin, error) {
		p := newMockPlugin(DeliveryMSE)
		p.healthy = true
		return p, nil
	})

	p, err := reg.Create(DeliveryMSE, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := p.Status()
	if !status.Healthy {
		t.Fatal("expected second factory to be used (healthy=true)")
	}
}
