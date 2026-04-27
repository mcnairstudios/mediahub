package source

import "context"

// Discoverable is implemented by source types that support network discovery
// (e.g. HDHomeRun devices via SSDP, SAT>IP tuners).
type Discoverable interface {
	Discover(ctx context.Context) ([]DiscoveredDevice, error)
}

// DiscoveredDevice represents a device found during network discovery.
type DiscoveredDevice struct {
	Host         string         `json:"host"`
	Identifier   string         `json:"identifier"`
	Name         string         `json:"name"`
	Model        string         `json:"model,omitempty"`
	AlreadyAdded bool           `json:"already_added"`
	Properties   map[string]any `json:"properties,omitempty"`
}

// Retunable is implemented by source types that can retune their connection
// without a full refresh (e.g. SAT>IP frequency changes).
type Retunable interface {
	Retune(ctx context.Context) error
}

// ConditionalRefresher is implemented by sources that support conditional
// fetching (e.g. HTTP ETag/If-Modified-Since), allowing skipped refreshes
// when upstream data hasn't changed.
type ConditionalRefresher interface {
	SupportsConditionalRefresh() bool
}

// VPNRoutable is implemented by sources whose streams must be routed
// through a VPN tunnel.
type VPNRoutable interface {
	UsesVPN() bool
}

// VODProvider is implemented by sources that provide video-on-demand
// content in addition to live streams.
type VODProvider interface {
	SupportsVOD() bool
	VODTypes() []string
}

// EPGProvider is implemented by sources that provide their own EPG data
// alongside streams.
type EPGProvider interface {
	ProvidesEPG() bool
}

// Clearable is implemented by sources that support clearing all cached
// data and state without deleting the source configuration itself.
type Clearable interface {
	Clear(ctx context.Context) error
}

// TLSStatus holds the mTLS enrollment state for a source.
type TLSStatus struct {
	Enrolled    bool   `json:"enrolled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// TLSProvider is implemented by sources that support mTLS client
// certificate authentication (e.g. tvpstreams with enrollment tokens).
type TLSProvider interface {
	TLSInfo() TLSStatus
}
