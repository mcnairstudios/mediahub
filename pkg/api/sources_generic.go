package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

// handleListSourceTypes returns descriptors for all registered source plugins.
// GET /api/source-types
func (s *Server) handleListSourceTypes(w http.ResponseWriter, r *http.Request) {
	plugins := s.deps.SourceReg.Plugins()
	descriptors := make([]source.PluginDescriptor, 0, len(plugins))
	for _, p := range plugins {
		descriptors = append(descriptors, p.Descriptor)
	}
	sort.Slice(descriptors, func(i, j int) bool {
		return descriptors[i].Type < descriptors[j].Type
	})
	httputil.RespondJSON(w, http.StatusOK, descriptors)
}

// handlePluginFrontendJS serves custom frontend JavaScript for a plugin.
// GET /api/source-types/{type}/frontend.js
func (s *Server) handlePluginFrontendJS(w http.ResponseWriter, r *http.Request) {
	st := source.SourceType(r.PathValue("type"))
	plugin := s.deps.SourceReg.Plugin(st)
	if plugin == nil || len(plugin.FrontendJS) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/javascript")
	w.Write(plugin.FrontendJS)
}

// handleGenericCreateSource creates a source config using the plugin's
// declared config fields.
// POST /api/source-plugins/{type}
func (s *Server) handleGenericCreateSource(w http.ResponseWriter, r *http.Request) {
	st := source.SourceType(r.PathValue("type"))
	plugin := s.deps.SourceReg.Plugin(st)
	if plugin == nil {
		httputil.RespondError(w, http.StatusBadRequest, "unknown source type")
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	name, _ := body["name"].(string)
	if name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	config := make(map[string]string)
	for _, field := range plugin.Descriptor.ConfigFields {
		val := extractConfigValue(body, field)
		if val == "" && field.Required {
			httputil.RespondError(w, http.StatusBadRequest, fmt.Sprintf("field %q is required", field.Key))
			return
		}
		if val == "" && len(field.Default) > 0 {
			// Default can be string, number, or array — coerce to string
			var defStr string
			if err := json.Unmarshal(field.Default, &defStr); err != nil {
				// Not a JSON string — use raw representation (e.g. "50", "[...]")
				defStr = string(field.Default)
			}
			val = defStr
		}
		if val != "" {
			config[field.Key] = val
		}
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      string(st),
		Name:      name,
		IsEnabled: true,
		Config:    config,
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, st, sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
		if epgID := config["epg_source_id"]; epgID != "" {
			s.AutoMatchStreamsToEPG(ctx, string(st), sc.ID, epgID)
		}
	}()

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

// handleGenericUpdateSource partially updates a source config.
// PUT /api/source-plugins/{type}/{id}
func (s *Server) handleGenericUpdateSource(w http.ResponseWriter, r *http.Request) {
	st := source.SourceType(r.PathValue("type"))
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	plugin := s.deps.SourceReg.Plugin(st)
	if plugin == nil {
		httputil.RespondError(w, http.StatusBadRequest, "unknown source type")
		return
	}

	existing, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get source")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	// Update top-level fields if present.
	if nameVal, ok := body["name"]; ok {
		if s, ok := nameVal.(string); ok {
			existing.Name = s
		}
	}
	if enabledVal, ok := body["is_enabled"]; ok {
		if b, ok := enabledVal.(bool); ok {
			existing.IsEnabled = b
		}
	}

	// Update config fields — only those present in the request body.
	for _, field := range plugin.Descriptor.ConfigFields {
		if _, present := body[field.Key]; !present {
			continue
		}
		val := extractConfigValue(body, field)
		if existing.Config == nil {
			existing.Config = make(map[string]string)
		}
		existing.Config[field.Key] = val
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

// handleGenericDeleteSource deletes a source and its streams.
// DELETE /api/source-plugins/{type}/{id}
func (s *Server) handleGenericDeleteSource(w http.ResponseWriter, r *http.Request) {
	st := source.SourceType(r.PathValue("type"))
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if s.deps.SourceReg.Plugin(st) == nil {
		httputil.RespondError(w, http.StatusBadRequest, "unknown source type")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(st), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handlePluginInteract dispatches an interact call to a WASM plugin.
// POST /api/source-plugins/{type}/interact
func (s *Server) handlePluginInteract(w http.ResponseWriter, r *http.Request) {
	st := source.SourceType(r.PathValue("type"))
	plugin := s.deps.SourceReg.Plugin(st)
	if plugin == nil {
		httputil.RespondError(w, http.StatusBadRequest, "unknown source type")
		return
	}

	if s.deps.PluginInteractor == nil {
		httputil.RespondError(w, http.StatusNotImplemented, "plugin interactions not available")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1*1024*1024))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	result, err := s.deps.PluginInteractor.Interact(r.Context(), string(st), body)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("interact failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(result)
}

// extractConfigValue reads a value from the request body for a given config
// field, converting it to a string suitable for the SourceConfig.Config map.
func extractConfigValue(body map[string]any, field source.ConfigField) string {
	raw, ok := body[field.Key]
	if !ok || raw == nil {
		return ""
	}

	switch field.Type {
	case source.FieldBool:
		switch v := raw.(type) {
		case bool:
			if v {
				return "true"
			}
			return "false"
		case string:
			return v
		default:
			return fmt.Sprintf("%v", v)
		}
	case source.FieldNumber:
		switch v := raw.(type) {
		case float64:
			// Use integer format if it's a whole number.
			if v == float64(int64(v)) {
				return fmt.Sprintf("%d", int64(v))
			}
			return fmt.Sprintf("%g", v)
		case string:
			return v
		default:
			return fmt.Sprintf("%v", v)
		}
	case source.FieldText, source.FieldPassword, source.FieldURL, source.FieldSelect, source.FieldHidden:
		if s, ok := raw.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", raw)
	case source.FieldCustom:
		// Complex values (arrays, objects) — JSON-encode them.
		b, err := json.Marshal(raw)
		if err != nil {
			return fmt.Sprintf("%v", raw)
		}
		return string(b)
	default:
		if s, ok := raw.(string); ok {
			return s
		}
		// Fall back to JSON for unknown/complex types.
		b, err := json.Marshal(raw)
		if err != nil {
			return fmt.Sprintf("%v", raw)
		}
		return string(b)
	}
}
