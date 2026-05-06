package source

import (
	"context"
	"encoding/json"
	"testing"
)

func TestConfigFieldJSONMarshal(t *testing.T) {
	field := ConfigField{
		Key:         "url",
		Label:       "Playlist URL",
		Type:        FieldURL,
		Required:    true,
		Placeholder: "https://example.com/playlist.m3u",
		HelpText:    "Enter the full M3U URL",
	}

	data, err := json.Marshal(field)
	if err != nil {
		t.Fatalf("failed to marshal ConfigField: %v", err)
	}

	var decoded ConfigField
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ConfigField: %v", err)
	}

	if decoded.Key != "url" {
		t.Fatalf("expected key url, got %s", decoded.Key)
	}
	if decoded.Type != FieldURL {
		t.Fatalf("expected type url, got %s", decoded.Type)
	}
	if !decoded.Required {
		t.Fatal("expected Required true")
	}
	if decoded.Placeholder != "https://example.com/playlist.m3u" {
		t.Fatalf("expected placeholder, got %s", decoded.Placeholder)
	}
}

func TestConfigFieldWithOptions(t *testing.T) {
	field := ConfigField{
		Key:   "refresh_interval",
		Label: "Refresh Interval",
		Type:  FieldSelect,
		Options: []Option{
			{Value: "1h", Label: "Every hour"},
			{Value: "24h", Label: "Every 24 hours"},
		},
	}

	data, err := json.Marshal(field)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ConfigField
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(decoded.Options))
	}
	if decoded.Options[0].Value != "1h" {
		t.Fatalf("expected first option value 1h, got %s", decoded.Options[0].Value)
	}
}

func TestPluginDescriptorJSON(t *testing.T) {
	desc := PluginDescriptor{
		Type:        TypeSpaceX,
		Label:       "Space Launches",
		ShortLabel:  "SPACE",
		Color:       "#1e88e5",
		Version:     "1.0.0",
		Description: "Launches from all space agencies",
		ConfigFields: []ConfigField{
			{Key: "api_key", Label: "API Key", Type: FieldPassword},
		},
	}

	data, err := json.Marshal(desc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded PluginDescriptor
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Type != TypeSpaceX {
		t.Fatalf("expected type spacex, got %s", decoded.Type)
	}
	if decoded.Label != "Space Launches" {
		t.Fatalf("expected label Space Launches, got %s", decoded.Label)
	}
	if decoded.Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %s", decoded.Version)
	}
	if len(decoded.ConfigFields) != 1 {
		t.Fatalf("expected 1 config field, got %d", len(decoded.ConfigFields))
	}
}

func TestPluginDescriptorEmptyConfigFields(t *testing.T) {
	desc := PluginDescriptor{
		Type:    TypeDemo,
		Label:   "Demo",
		Version: "1.0.0",
	}

	data, err := json.Marshal(desc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded PluginDescriptor
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ConfigFields != nil {
		t.Fatalf("expected nil config fields, got %v", decoded.ConfigFields)
	}
}

func TestRegistryRegisterPlugin(t *testing.T) {
	reg := NewRegistry()

	reg.RegisterPlugin(PluginRegistration{
		Descriptor: PluginDescriptor{
			Type:    TypeSpaceX,
			Label:   "Space Launches",
			Version: "1.0.0",
		},
		Factory: func(_ context.Context, id string) (Source, error) {
			return &mockSource{info: SourceInfo{ID: id, Type: TypeSpaceX}}, nil
		},
	})

	plugin := reg.Plugin(TypeSpaceX)
	if plugin == nil {
		t.Fatal("expected plugin to be registered")
	}
	if plugin.Descriptor.Label != "Space Launches" {
		t.Fatalf("expected label Space Launches, got %s", plugin.Descriptor.Label)
	}

	// Factory should also be registered via RegisterPlugin.
	src, err := reg.Create(context.Background(), TypeSpaceX, "src-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Type() != TypeSpaceX {
		t.Fatalf("expected type spacex, got %s", src.Type())
	}
}

func TestRegistryRegisterPluginWithoutFactory(t *testing.T) {
	reg := NewRegistry()

	// Register a factory separately.
	reg.Register(TypeDemo, func(_ context.Context, id string) (Source, error) {
		return &mockSource{info: SourceInfo{ID: id, Type: TypeDemo}}, nil
	})

	// Register plugin descriptor only (no factory).
	reg.RegisterPlugin(PluginRegistration{
		Descriptor: PluginDescriptor{
			Type:    TypeDemo,
			Label:   "Demo",
			Version: "1.0.0",
		},
	})

	// Plugin should be available.
	plugin := reg.Plugin(TypeDemo)
	if plugin == nil {
		t.Fatal("expected plugin to be registered")
	}

	// Factory from Register() should still work.
	src, err := reg.Create(context.Background(), TypeDemo, "src-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Type() != TypeDemo {
		t.Fatalf("expected type demo, got %s", src.Type())
	}
}

func TestRegistryPluginNotFound(t *testing.T) {
	reg := NewRegistry()
	if plugin := reg.Plugin(TypeM3U); plugin != nil {
		t.Fatal("expected nil for unregistered plugin")
	}
}

func TestRegistryPlugins(t *testing.T) {
	reg := NewRegistry()

	reg.RegisterPlugin(PluginRegistration{
		Descriptor: PluginDescriptor{Type: TypeSpaceX, Label: "Space", Version: "1.0.0"},
	})
	reg.RegisterPlugin(PluginRegistration{
		Descriptor: PluginDescriptor{Type: TypeDemo, Label: "Demo", Version: "1.0.0"},
	})
	reg.RegisterPlugin(PluginRegistration{
		Descriptor: PluginDescriptor{Type: TypeM3U, Label: "M3U", Version: "1.0.0"},
	})

	plugins := reg.Plugins()
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(plugins))
	}

	// Should be sorted by type.
	if plugins[0].Descriptor.Type != TypeDemo {
		t.Fatalf("expected first plugin to be demo, got %s", plugins[0].Descriptor.Type)
	}
	if plugins[1].Descriptor.Type != TypeM3U {
		t.Fatalf("expected second plugin to be m3u, got %s", plugins[1].Descriptor.Type)
	}
	if plugins[2].Descriptor.Type != TypeSpaceX {
		t.Fatalf("expected third plugin to be spacex, got %s", plugins[2].Descriptor.Type)
	}
}

func TestRegistryPluginsEmpty(t *testing.T) {
	reg := NewRegistry()
	plugins := reg.Plugins()
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestCustomRouteStoresHandler(t *testing.T) {
	called := false
	handler := func() { called = true }

	route := CustomRoute{
		Method:  "GET",
		Pattern: "places",
		Handler: handler,
	}

	if route.Method != "GET" {
		t.Fatalf("expected method GET, got %s", route.Method)
	}
	if route.Pattern != "places" {
		t.Fatalf("expected pattern places, got %s", route.Pattern)
	}

	// Verify the handler is stored.
	if fn, ok := route.Handler.(func()); ok {
		fn()
	}
	if !called {
		t.Fatal("expected handler to be callable")
	}
}

func TestFieldTypeConstants(t *testing.T) {
	types := []FieldType{
		FieldText, FieldPassword, FieldURL, FieldNumber,
		FieldBool, FieldSelect, FieldHidden, FieldCustom,
	}
	expected := []string{
		"text", "password", "url", "number",
		"bool", "select", "hidden", "custom",
	}
	for i, ft := range types {
		if string(ft) != expected[i] {
			t.Fatalf("expected %s, got %s", expected[i], ft)
		}
	}
}
