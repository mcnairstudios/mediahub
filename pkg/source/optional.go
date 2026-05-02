package source

import "context"

type Discoverable interface {
	Discover(ctx context.Context) ([]DiscoveredDevice, error)
}

type DiscoveredDevice struct {
	Host         string         `json:"host"`
	Identifier   string         `json:"identifier"`
	Name         string         `json:"name"`
	Model        string         `json:"model,omitempty"`
	AlreadyAdded bool           `json:"already_added"`
	Properties   map[string]any `json:"properties,omitempty"`
}

type Retunable interface {
	Retune(ctx context.Context) error
}

type ConditionalRefresher interface {
	SupportsConditionalRefresh() bool
}

type VPNRoutable interface {
	UsesVPN() bool
}

type VODProvider interface {
	SupportsVOD() bool
	VODTypes() []string
}

type EPGProvider interface {
	ProvidesEPG() bool
}

type Clearable interface {
	Clear(ctx context.Context) error
}

type TLSStatus struct {
	Enrolled    bool   `json:"enrolled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type TLSProvider interface {
	TLSInfo() TLSStatus
}

type AccountInfoProvider interface {
	GetAccountInfo(ctx context.Context) (any, error)
}
