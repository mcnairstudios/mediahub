package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

func newTestEnvWithPlugins(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)

	// Register a test plugin with config fields.
	env.server.deps.SourceReg.RegisterPlugin(source.PluginRegistration{
		Descriptor: source.PluginDescriptor{
			Type:        source.TypeSpaceX,
			Label:       "Space Launches",
			ShortLabel:  "SPACE",
			Color:       "#1e88e5",
			Version:     "1.0.0",
			Description: "Test space launches plugin",
		},
	})

	env.server.deps.SourceReg.RegisterPlugin(source.PluginRegistration{
		Descriptor: source.PluginDescriptor{
			Type:        source.TypeDemo,
			Label:       "Demo Streams",
			ShortLabel:  "DEMO",
			Color:       "#607d8b",
			Version:     "1.0.0",
			Description: "Demo streams for testing",
			ConfigFields: []source.ConfigField{
				{Key: "max_streams", Label: "Max Streams", Type: source.FieldNumber, Default: json.RawMessage(`"10"`)},
				{Key: "quality", Label: "Quality", Type: source.FieldSelect, Required: true, Options: []source.Option{
					{Value: "low", Label: "Low"},
					{Value: "high", Label: "High"},
				}},
			},
		},
		FrontendJS: []byte("console.log('demo plugin');"),
	})

	return env
}

func TestHandleListSourceTypes(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	resp := env.request("GET", "/api/source-types", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var descriptors []source.PluginDescriptor
	decodeBody(resp, &descriptors)

	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descriptors))
	}

	// Should be sorted by type.
	if descriptors[0].Type != source.TypeDemo {
		t.Fatalf("expected first type demo, got %s", descriptors[0].Type)
	}
	if descriptors[1].Type != source.TypeSpaceX {
		t.Fatalf("expected second type spacex, got %s", descriptors[1].Type)
	}
}

func TestHandleListSourceTypesUnauthenticated(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	resp := env.request("GET", "/api/source-types", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHandleGenericCreateSource(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	body := map[string]any{
		"name":        "My Demo Source",
		"quality":     "high",
		"max_streams": 5,
	}

	resp := env.request("POST", "/api/source-plugins/demo", body, env.adminToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	if sc.Name != "My Demo Source" {
		t.Fatalf("expected name 'My Demo Source', got %s", sc.Name)
	}
	if sc.Type != "demo" {
		t.Fatalf("expected type demo, got %s", sc.Type)
	}
	if !sc.IsEnabled {
		t.Fatal("expected is_enabled true")
	}
	if sc.Config["quality"] != "high" {
		t.Fatalf("expected quality high, got %s", sc.Config["quality"])
	}
	if sc.Config["max_streams"] != "5" {
		t.Fatalf("expected max_streams 5, got %s", sc.Config["max_streams"])
	}
}

func TestHandleGenericCreateSourceMissingName(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	body := map[string]any{
		"quality": "high",
	}

	resp := env.request("POST", "/api/source-plugins/demo", body, env.adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleGenericCreateSourceMissingRequired(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	body := map[string]any{
		"name": "No Quality",
		// "quality" is required but missing.
	}

	resp := env.request("POST", "/api/source-plugins/demo", body, env.adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleGenericCreateSourceUnknownType(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	body := map[string]any{"name": "Test"}
	resp := env.request("POST", "/api/source-plugins/nonexistent", body, env.adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleGenericCreateSourceDefault(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	body := map[string]any{
		"name":    "Defaults Test",
		"quality": "low",
		// max_streams not provided, should default to "10".
	}

	resp := env.request("POST", "/api/source-plugins/demo", body, env.adminToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	if sc.Config["max_streams"] != "10" {
		t.Fatalf("expected default max_streams 10, got %s", sc.Config["max_streams"])
	}
}

func TestHandleGenericUpdateSource(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	// Create a source first.
	ctx := context.Background()
	sc := &sourceconfig.SourceConfig{
		ID:        "update-test-1",
		Type:      "demo",
		Name:      "Original Name",
		IsEnabled: true,
		Config:    map[string]string{"quality": "low", "max_streams": "5"},
	}
	if err := env.server.deps.SourceConfigStore.Create(ctx, sc); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	// Partial update: change name and quality, leave max_streams.
	body := map[string]any{
		"name":    "Updated Name",
		"quality": "high",
	}

	resp := env.request("PUT", "/api/source-plugins/demo/update-test-1", body, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var updated sourceconfig.SourceConfig
	decodeBody(resp, &updated)

	if updated.Name != "Updated Name" {
		t.Fatalf("expected name Updated Name, got %s", updated.Name)
	}
	if updated.Config["quality"] != "high" {
		t.Fatalf("expected quality high, got %s", updated.Config["quality"])
	}
	if updated.Config["max_streams"] != "5" {
		t.Fatalf("expected max_streams 5 (unchanged), got %s", updated.Config["max_streams"])
	}
}

func TestHandleGenericUpdateSourceNotFound(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	body := map[string]any{"name": "test"}
	resp := env.request("PUT", "/api/source-plugins/demo/nonexistent", body, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleGenericUpdateSourceIsEnabled(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	ctx := context.Background()
	sc := &sourceconfig.SourceConfig{
		ID:        "enabled-test-1",
		Type:      "demo",
		Name:      "Enabled Test",
		IsEnabled: true,
		Config:    map[string]string{"quality": "high"},
	}
	if err := env.server.deps.SourceConfigStore.Create(ctx, sc); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	body := map[string]any{"is_enabled": false}
	resp := env.request("PUT", "/api/source-plugins/demo/enabled-test-1", body, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var updated sourceconfig.SourceConfig
	decodeBody(resp, &updated)
	if updated.IsEnabled {
		t.Fatal("expected is_enabled false")
	}
}

func TestHandleGenericDeleteSource(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	ctx := context.Background()
	sc := &sourceconfig.SourceConfig{
		ID:        "delete-test-1",
		Type:      "demo",
		Name:      "To Delete",
		IsEnabled: true,
		Config:    map[string]string{},
	}
	if err := env.server.deps.SourceConfigStore.Create(ctx, sc); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	resp := env.request("DELETE", "/api/source-plugins/demo/delete-test-1", nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify it's gone.
	got, err := env.server.deps.SourceConfigStore.Get(ctx, "delete-test-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected source to be deleted")
	}
}

func TestHandleGenericDeleteSourceUnknownType(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	resp := env.request("DELETE", "/api/source-plugins/nonexistent/some-id", nil, env.adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandlePluginFrontendJS(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	resp := env.request("GET", "/api/source-types/demo/frontend.js", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/javascript" {
		t.Fatalf("expected content-type application/javascript, got %s", ct)
	}

	var buf [1024]byte
	n, _ := resp.Body.Read(buf[:])
	resp.Body.Close()
	body := string(buf[:n])
	if body != "console.log('demo plugin');" {
		t.Fatalf("unexpected JS body: %s", body)
	}
}

func TestHandlePluginFrontendJSNotFound(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	// SpaceX has no frontend JS.
	resp := env.request("GET", "/api/source-types/spacex/frontend.js", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandlePluginFrontendJSUnknownType(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	resp := env.request("GET", "/api/source-types/nonexistent/frontend.js", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleGenericCreateSourceNonAdmin(t *testing.T) {
	env := newTestEnvWithPlugins(t)
	defer env.close()

	body := map[string]any{
		"name":    "Test",
		"quality": "high",
	}

	resp := env.request("POST", "/api/source-plugins/demo", body, env.standardToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestExtractConfigValueTypes(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]any
		field    source.ConfigField
		expected string
	}{
		{
			name:     "string text",
			body:     map[string]any{"url": "http://example.com"},
			field:    source.ConfigField{Key: "url", Type: source.FieldText},
			expected: "http://example.com",
		},
		{
			name:     "bool true",
			body:     map[string]any{"enabled": true},
			field:    source.ConfigField{Key: "enabled", Type: source.FieldBool},
			expected: "true",
		},
		{
			name:     "bool false",
			body:     map[string]any{"enabled": false},
			field:    source.ConfigField{Key: "enabled", Type: source.FieldBool},
			expected: "false",
		},
		{
			name:     "number integer",
			body:     map[string]any{"port": float64(8080)},
			field:    source.ConfigField{Key: "port", Type: source.FieldNumber},
			expected: "8080",
		},
		{
			name:     "number float",
			body:     map[string]any{"rate": 1.5},
			field:    source.ConfigField{Key: "rate", Type: source.FieldNumber},
			expected: "1.5",
		},
		{
			name:     "custom array",
			body:     map[string]any{"places": []any{map[string]any{"id": "1", "name": "London"}}},
			field:    source.ConfigField{Key: "places", Type: source.FieldCustom},
			expected: `[{"id":"1","name":"London"}]`,
		},
		{
			name:     "missing key",
			body:     map[string]any{},
			field:    source.ConfigField{Key: "missing", Type: source.FieldText},
			expected: "",
		},
		{
			name:     "nil value",
			body:     map[string]any{"key": nil},
			field:    source.ConfigField{Key: "key", Type: source.FieldText},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractConfigValue(tt.body, tt.field)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractConfigValueCustomJSON(t *testing.T) {
	body := map[string]any{
		"places": []any{
			map[string]any{"id": "abc", "name": "Paris"},
			map[string]any{"id": "def", "name": "Berlin"},
		},
	}
	field := source.ConfigField{Key: "places", Type: source.FieldCustom}

	result := extractConfigValue(body, field)

	var parsed []map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse result as JSON: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 items, got %d", len(parsed))
	}
	if parsed[0]["name"] != "Paris" {
		t.Fatalf("expected first place Paris, got %s", parsed[0]["name"])
	}
}
