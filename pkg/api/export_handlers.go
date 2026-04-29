package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
)

type ExportData struct {
	Version        int                       `json:"version"`
	Scope          string                    `json:"scope"`
	ExportedAt     time.Time                 `json:"exported_at"`
	ChannelGroups  []channel.Group           `json:"channel_groups,omitempty"`
	Channels       []ExportChannel           `json:"channels,omitempty"`
	Clients        []client.Client           `json:"clients,omitempty"`
	SourceProfiles []sourceprofile.Profile   `json:"source_profiles,omitempty"`
	SourceConfigs  []sourceconfig.SourceConfig `json:"source_configs,omitempty"`
	EPGSources     []epg.Source              `json:"epg_sources,omitempty"`
	Settings       map[string]string         `json:"settings,omitempty"`
	Users          []ExportUser              `json:"users,omitempty"`
}

type ExportChannel struct {
	Name      string   `json:"name"`
	Number    int      `json:"number"`
	GroupName string   `json:"group_name,omitempty"`
	LogoURL   string   `json:"logo_url,omitempty"`
	TvgID     string   `json:"tvg_id,omitempty"`
	IsEnabled bool     `json:"is_enabled"`
	StreamIDs []string `json:"stream_ids,omitempty"`
}

type ExportUser struct {
	Username string    `json:"username"`
	Email    string    `json:"email,omitempty"`
	Role     auth.Role `json:"role"`
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "channels"
	}

	ctx := r.Context()
	data := &ExportData{
		Version:    1,
		Scope:      scope,
		ExportedAt: time.Now(),
	}

	groups, err := s.deps.GroupStore.List(ctx)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "export failed: "+err.Error())
		return
	}
	data.ChannelGroups = groups

	groupNameMap := make(map[string]string, len(groups))
	for _, g := range groups {
		groupNameMap[g.ID] = g.Name
	}

	channels, err := s.deps.ChannelStore.List(ctx)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "export failed: "+err.Error())
		return
	}

	for _, ch := range channels {
		ec := ExportChannel{
			Name:      ch.Name,
			Number:    ch.Number,
			LogoURL:   ch.LogoURL,
			TvgID:     ch.TvgID,
			IsEnabled: ch.IsEnabled,
			StreamIDs: ch.StreamIDs,
		}
		if ch.GroupID != "" {
			ec.GroupName = groupNameMap[ch.GroupID]
		}
		data.Channels = append(data.Channels, ec)
	}

	if scope == "full" {
		if s.deps.ClientStore != nil {
			clients, err := s.deps.ClientStore.List(ctx)
			if err == nil {
				data.Clients = clients
			}
		}

		if s.deps.SourceProfileStore != nil {
			profiles, err := s.deps.SourceProfileStore.List(ctx)
			if err == nil {
				data.SourceProfiles = profiles
			}
		}

		if s.deps.SourceConfigStore != nil {
			configs, err := s.deps.SourceConfigStore.List(ctx)
			if err == nil {
				data.SourceConfigs = configs
			}
		}

		if s.deps.EPGSourceStore != nil {
			sources, err := s.deps.EPGSourceStore.List(ctx)
			if err == nil {
				data.EPGSources = sources
			}
		}

		if s.deps.SettingsStore != nil {
			settings, err := s.deps.SettingsStore.List(ctx)
			if err == nil {
				filtered := make(map[string]string, len(settings))
				for k, v := range settings {
					if apiSettableKeys[k] {
						filtered[k] = v
					}
				}
				data.Settings = filtered
			}
		}

		if s.deps.AuthService != nil {
			users, err := s.deps.AuthService.ListUsers(ctx)
			if err == nil {
				for _, u := range users {
					data.Users = append(data.Users, ExportUser{
						Username: u.Username,
						Email:    u.Email,
						Role:     u.Role,
					})
				}
			}
		}
	}

	httputil.RespondJSON(w, http.StatusOK, data)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var data ExportData
	if err := httputil.DecodeJSON(r, &data); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid import data")
		return
	}

	if data.Version != 1 {
		httputil.RespondError(w, http.StatusBadRequest, fmt.Sprintf("unsupported export version: %d", data.Version))
		return
	}

	ctx := r.Context()
	var imported int

	existingGroups, _ := s.deps.GroupStore.List(ctx)
	groupNameToID := make(map[string]string)
	for _, g := range existingGroups {
		groupNameToID[g.Name] = g.ID
	}
	for _, g := range data.ChannelGroups {
		if _, exists := groupNameToID[g.Name]; exists {
			continue
		}
		ng := &channel.Group{ID: g.ID, Name: g.Name}
		if ng.ID == "" {
			ng.ID = generateID()
		}
		if err := s.deps.GroupStore.Create(ctx, ng); err == nil {
			groupNameToID[ng.Name] = ng.ID
			imported++
		}
	}

	existingChannels, _ := s.deps.ChannelStore.List(ctx)
	channelNameSet := make(map[string]bool, len(existingChannels))
	for _, c := range existingChannels {
		channelNameSet[c.Name] = true
	}
	for _, ec := range data.Channels {
		if channelNameSet[ec.Name] {
			continue
		}
		ch := &channel.Channel{
			ID:        generateID(),
			Name:      ec.Name,
			Number:    ec.Number,
			LogoURL:   ec.LogoURL,
			TvgID:     ec.TvgID,
			IsEnabled: ec.IsEnabled,
			StreamIDs: ec.StreamIDs,
		}
		if ec.GroupName != "" {
			if gid, ok := groupNameToID[ec.GroupName]; ok {
				ch.GroupID = gid
			}
		}
		if err := s.deps.ChannelStore.Create(ctx, ch); err == nil {
			imported++
		}
	}

	if len(data.Clients) > 0 && s.deps.ClientStore != nil {
		existingClients, _ := s.deps.ClientStore.List(ctx)
		clientNameSet := make(map[string]bool, len(existingClients))
		for _, c := range existingClients {
			clientNameSet[c.Name] = true
		}
		for _, c := range data.Clients {
			if clientNameSet[c.Name] {
				continue
			}
			if c.IsSystem {
				continue
			}
			nc := c
			nc.ID = generateID()
			if err := s.deps.ClientStore.Create(ctx, &nc); err == nil {
				imported++
			}
		}
	}

	if len(data.SourceProfiles) > 0 && s.deps.SourceProfileStore != nil {
		existingProfiles, _ := s.deps.SourceProfileStore.List(ctx)
		profileNameSet := make(map[string]bool, len(existingProfiles))
		for _, p := range existingProfiles {
			profileNameSet[p.Name] = true
		}
		for _, p := range data.SourceProfiles {
			if profileNameSet[p.Name] {
				continue
			}
			np := p
			np.ID = generateID()
			if err := s.deps.SourceProfileStore.Create(ctx, &np); err == nil {
				imported++
			}
		}
	}

	if len(data.SourceConfigs) > 0 && s.deps.SourceConfigStore != nil {
		existingConfigs, _ := s.deps.SourceConfigStore.List(ctx)
		configNameSet := make(map[string]bool, len(existingConfigs))
		for _, c := range existingConfigs {
			configNameSet[c.Name] = true
		}
		for _, c := range data.SourceConfigs {
			if configNameSet[c.Name] {
				continue
			}
			nc := c
			nc.ID = generateID()
			if err := s.deps.SourceConfigStore.Create(ctx, &nc); err == nil {
				imported++
			}
		}
	}

	if len(data.EPGSources) > 0 && s.deps.EPGSourceStore != nil {
		existingSources, _ := s.deps.EPGSourceStore.List(ctx)
		sourceNameSet := make(map[string]bool, len(existingSources))
		for _, es := range existingSources {
			sourceNameSet[es.Name] = true
		}
		for _, es := range data.EPGSources {
			if sourceNameSet[es.Name] {
				continue
			}
			ns := es
			ns.ID = generateID()
			if err := s.deps.EPGSourceStore.Create(ctx, &ns); err == nil {
				imported++
			}
		}
	}

	if len(data.Settings) > 0 && s.deps.SettingsStore != nil {
		for key, value := range data.Settings {
			if apiSettableKeys[key] {
				if err := s.deps.SettingsStore.Set(ctx, key, value); err == nil {
					imported++
				}
			}
		}
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"message":  "import complete",
		"imported": imported,
	})
}

type BucketClearer interface {
	ClearBucket(name string) error
	ClearAll() error
}

func (s *Server) handleSoftReset(w http.ResponseWriter, r *http.Request) {
	clearer, ok := s.deps.DBClearer.(BucketClearer)
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "reset not supported")
		return
	}

	softBuckets := []string{
		"channels", "groups", "streams", "epg_programs",
		"clients", "source_profiles", "favorites", "recordings",
	}

	for _, bucket := range softBuckets {
		if err := clearer.ClearBucket(bucket); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "soft reset failed: "+err.Error())
			return
		}
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "soft reset complete"})
}

func (s *Server) handleHardReset(w http.ResponseWriter, r *http.Request) {
	clearer, ok := s.deps.DBClearer.(BucketClearer)
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "reset not supported")
		return
	}

	if err := clearer.ClearAll(); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "hard reset failed: "+err.Error())
		return
	}

	ctx := r.Context()
	if _, err := s.deps.AuthService.CreateUser(ctx, "admin", "admin", "", auth.RoleAdmin); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create default admin: "+err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "hard reset complete"})
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
