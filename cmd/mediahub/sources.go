package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/source"
	hdhrsource "github.com/mcnairstudios/mediahub/pkg/source/hdhr"
	m3usource "github.com/mcnairstudios/mediahub/pkg/source/m3u"
	satipsource "github.com/mcnairstudios/mediahub/pkg/source/satip"
	tvpstreamssource "github.com/mcnairstudios/mediahub/pkg/source/tvpstreams"
	xstreamsource "github.com/mcnairstudios/mediahub/pkg/source/xtream"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

// jsonDefault wraps a string as a JSON string value for ConfigField.Default.
func jsonDefault(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

type sourceDeps struct {
	SourceConfigStore sourceconfig.Store
	StreamStore       store.StreamStore
	SettingsStore     store.SettingsStore
	Config            *config.Config
	WGService         *wg.Service
	TMDBCache         *tmdbcache.Cache
	OnRefreshDone     func(string, string, int)
}

func registerSources(reg *source.Registry, deps sourceDeps) {
	reg.Register(source.TypeM3U, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		m3uCfg := m3usource.Config{
			ID:            sc.ID,
			Name:          sc.Name,
			URL:           sc.Config["url"],
			IsEnabled:     sc.IsEnabled,
			UseWireGuard:  sc.Config["use_wireguard"] == "true",
			WGProfileID:   sc.Config["wg_profile_id"],
			UserAgent:     deps.Config.UserAgent,
			BypassHeader:  deps.Config.BypassHeader,
			BypassSecret:  deps.Config.BypassSecret,
			InitialETag:   sc.Config["etag"],
			StreamStore:   deps.StreamStore,
			OnRefreshDone: deps.OnRefreshDone,
		}
		m3uCfg.WGClient = resolveWGClient(deps.WGService, m3uCfg.UseWireGuard, m3uCfg.WGProfileID)
		return m3usource.New(m3uCfg), nil
	})

	reg.Register(source.TypeTVPStreams, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		tvpCfg := tvpstreamssource.Config{
			ID:              sc.ID,
			Name:            sc.Name,
			URL:             sc.Config["url"],
			IsEnabled:       sc.IsEnabled,
			UseWireGuard:    sc.Config["use_wireguard"] == "true",
			DataDir:         deps.Config.DataDir,
			EnrollmentToken: sc.Config["enrollment_token"],
			TLSEnrolled:     sc.Config["tls_enrolled"] == "true",
			BypassHeader:    deps.Config.BypassHeader,
			BypassSecret:    deps.Config.BypassSecret,
			StreamStore:     deps.StreamStore,
			TMDBCache:       deps.TMDBCache,
			InitialETag:     sc.Config["etag"],
			OnRefreshDone:   deps.OnRefreshDone,
			OnEnrolled: func(sourceID string) error {
				scUpd, err := deps.SourceConfigStore.Get(ctx, sourceID)
				if err != nil || scUpd == nil {
					return err
				}
				scUpd.Config["tls_enrolled"] = "true"
				scUpd.Config["enrollment_token"] = ""
				return deps.SourceConfigStore.Update(ctx, scUpd)
			},
		}
		wgProfileID := sc.Config["wg_profile_id"]
		tvpCfg.WGClient = resolveWGClient(deps.WGService, tvpCfg.UseWireGuard, wgProfileID)
		return tvpstreamssource.New(tvpCfg), nil
	})

	reg.Register(source.TypeXtream, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		maxStreams := 0
		if v := sc.Config["max_streams"]; v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				maxStreams = n
			}
		}
		xtCfg := xstreamsource.Config{
			ID:            sc.ID,
			Name:          sc.Name,
			Server:        sc.Config["server"],
			Username:      sc.Config["username"],
			Password:      sc.Config["password"],
			IsEnabled:     sc.IsEnabled,
			UseWireGuard:  sc.Config["use_wireguard"] == "true",
			MaxStreams:    maxStreams,
			StreamStore:   deps.StreamStore,
			OnRefreshDone: deps.OnRefreshDone,
		}
		xtCfg.WGClient = resolveWGClient(deps.WGService, xtCfg.UseWireGuard, sc.Config["wg_profile_id"])
		return xstreamsource.New(xtCfg), nil
	})

	reg.Register(source.TypeHDHR, func(ctx context.Context, sourceID string) (source.Source, error) {
		if sourceID == "" {
			return hdhrsource.New(hdhrsource.Config{
				StreamStore: deps.StreamStore,
			}), nil
		}
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		var devices []hdhrsource.Device
		if devicesJSON := sc.Config["devices"]; devicesJSON != "" {
			if jsonErr := json.Unmarshal([]byte(devicesJSON), &devices); jsonErr != nil {
				log.Printf("hdhr: failed to parse devices for %s: %v", sc.Name, jsonErr)
			}
		}
		hdhrCfg := hdhrsource.Config{
			ID:          sc.ID,
			Name:        sc.Name,
			IsEnabled:   sc.IsEnabled,
			Devices:     devices,
			StreamStore: deps.StreamStore,
		}
		return hdhrsource.New(hdhrCfg), nil
	})

	reg.Register(source.TypeSATIP, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		httpPort := 8875
		if v := sc.Config["http_port"]; v != "" {
			if n, pErr := strconv.Atoi(v); pErr == nil {
				httpPort = n
			}
		}
		maxStreams := 0
		if v := sc.Config["max_streams"]; v != "" {
			if n, pErr := strconv.Atoi(v); pErr == nil {
				maxStreams = n
			}
		}
		diseqcSource := 0
		if ds := sc.Config["diseqc_source"]; ds != "" {
			fmt.Sscanf(ds, "%d", &diseqcSource)
		}
		satipCfg := satipsource.Config{
			ID:              sc.ID,
			Name:            sc.Name,
			Host:            sc.Config["host"],
			HTTPPort:        httpPort,
			IsEnabled:       sc.IsEnabled,
			MaxStreams:       maxStreams,
			TransmitterFile: sc.Config["transmitter_file"],
			DiSEqCSource:    diseqcSource,
			StreamStore:     deps.StreamStore,
		}
		return satipsource.New(satipCfg), nil
	})

	// Demo, Space Launches, Radio Garden, and Trailers are WASM plugins
	// loaded from MEDIAHUB_PLUGINS_DIR — not compiled-in.

	// Register plugin descriptors for the compiled-in source types so
	// /api/source-types returns them. WASM plugins register themselves.
	registerBuiltinDescriptors(reg)
}

// registerBuiltinDescriptors adds PluginDescriptor metadata for every
// built-in source type. Factories remain set via Register() above; the
// descriptors provide UI metadata for the generic plugin infrastructure.
func registerBuiltinDescriptors(reg *source.Registry) {
	descriptors := []source.PluginDescriptor{
		{
			Type:        source.TypeM3U,
			Label:       "M3U Playlist",
			ShortLabel:  "M3U",
			Color:       "#4caf50",
			Version:     "1.0.0",
			Description: "Import channels from an M3U playlist URL",
			ConfigFields: []source.ConfigField{
				{Key: "url", Label: "Playlist URL", Type: source.FieldURL, Required: true, Placeholder: "https://example.com/playlist.m3u"},
				{Key: "username", Label: "Username", Type: source.FieldText, HelpText: "Optional credentials for protected playlists"},
				{Key: "password", Label: "Password", Type: source.FieldPassword},
				{Key: "use_wireguard", Label: "Use WireGuard", Type: source.FieldBool},
				{Key: "wg_profile_id", Label: "WireGuard Profile", Type: source.FieldSelect},
				{Key: "refresh_interval", Label: "Refresh Interval", Type: source.FieldSelect, Default: jsonDefault("24h"), Options: []source.Option{
					{Value: "1h", Label: "Every hour"},
					{Value: "6h", Label: "Every 6 hours"},
					{Value: "12h", Label: "Every 12 hours"},
					{Value: "24h", Label: "Every 24 hours"},
				}},
				{Key: "source_profile_id", Label: "Source Profile", Type: source.FieldSelect},
				{Key: "epg_source_id", Label: "EPG Source", Type: source.FieldSelect},
			},
		},
		{
			Type:        source.TypeXtream,
			Label:       "Xtream Codes",
			ShortLabel:  "XTREAM",
			Color:       "#ff9800",
			Version:     "1.0.0",
			Description: "Connect to an Xtream Codes compatible server",
			ConfigFields: []source.ConfigField{
				{Key: "server", Label: "Server URL", Type: source.FieldURL, Required: true, Placeholder: "http://example.com:8080"},
				{Key: "username", Label: "Username", Type: source.FieldText, Required: true},
				{Key: "password", Label: "Password", Type: source.FieldPassword, Required: true},
				{Key: "use_wireguard", Label: "Use WireGuard", Type: source.FieldBool},
				{Key: "wg_profile_id", Label: "WireGuard Profile", Type: source.FieldSelect},
				{Key: "max_streams", Label: "Max Streams", Type: source.FieldNumber, Default: jsonDefault("0"), HelpText: "0 = unlimited"},
				{Key: "refresh_interval", Label: "Refresh Interval", Type: source.FieldSelect, Default: jsonDefault("24h"), Options: []source.Option{
					{Value: "1h", Label: "Every hour"},
					{Value: "6h", Label: "Every 6 hours"},
					{Value: "12h", Label: "Every 12 hours"},
					{Value: "24h", Label: "Every 24 hours"},
				}},
				{Key: "source_profile_id", Label: "Source Profile", Type: source.FieldSelect},
				{Key: "epg_source_id", Label: "EPG Source", Type: source.FieldSelect},
			},
		},
		{
			Type:        source.TypeTVPStreams,
			Label:       "TVProxy Streams",
			ShortLabel:  "TVPROXY",
			Color:       "#9c27b0",
			Version:     "1.0.0",
			Description: "Connect to a tvproxy-streams instance via mTLS",
			ConfigFields: []source.ConfigField{
				{Key: "url", Label: "Server URL", Type: source.FieldURL, Required: true, Placeholder: "https://tvproxy.example.com"},
				{Key: "enrollment_token", Label: "Enrollment Token", Type: source.FieldPassword, HelpText: "One-time token for mTLS enrollment"},
				{Key: "use_wireguard", Label: "Use WireGuard", Type: source.FieldBool},
				{Key: "wg_profile_id", Label: "WireGuard Profile", Type: source.FieldSelect},
				{Key: "source_profile_id", Label: "Source Profile", Type: source.FieldSelect},
				{Key: "epg_source_id", Label: "EPG Source", Type: source.FieldSelect},
			},
		},
		{
			Type:        source.TypeHDHR,
			Label:       "HDHomeRun",
			ShortLabel:  "HDHR",
			Color:       "#00bcd4",
			Version:     "1.0.0",
			Description: "Discover and use HDHomeRun network tuners",
			ConfigFields: []source.ConfigField{
				{Key: "devices", Label: "Devices", Type: source.FieldCustom, Component: "hdhr-devices", HelpText: "Manage devices via the HDHomeRun discovery UI"},
			},
		},
		{
			Type:        source.TypeSATIP,
			Label:       "SAT>IP",
			ShortLabel:  "SATIP",
			Color:       "#795548",
			Version:     "1.0.0",
			Description: "Connect to a SAT>IP DVB tuner for satellite, cable, or terrestrial TV",
			ConfigFields: []source.ConfigField{
				{Key: "host", Label: "Host", Type: source.FieldText, Required: true, Placeholder: "192.168.1.100"},
				{Key: "http_port", Label: "HTTP Port", Type: source.FieldNumber, Default: jsonDefault("8875")},
				{Key: "max_streams", Label: "Max Streams", Type: source.FieldNumber, Default: jsonDefault("0"), HelpText: "0 = unlimited"},
				{Key: "transmitter_file", Label: "Transmitter File", Type: source.FieldText, HelpText: "Path to transmitter definition file"},
				{Key: "diseqc_source", Label: "DiSEqC Source", Type: source.FieldNumber, Default: jsonDefault("0")},
			},
		},
		// Demo, Trailers, Space Launches, and Radio Garden are WASM plugins.
		// Their descriptors are loaded from .wasm files in MEDIAHUB_PLUGINS_DIR.
	}

	for _, desc := range descriptors {
		// Only register the descriptor — factories are already set via
		// Register() calls above. If a plugin entry already exists
		// (e.g. from an init() self-registration), update its descriptor.
		if existing := reg.Plugin(desc.Type); existing != nil {
			existing.Descriptor = desc
			continue
		}
		reg.RegisterPlugin(source.PluginRegistration{
			Descriptor: desc,
		})
	}
}
