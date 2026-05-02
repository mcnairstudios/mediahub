package sourceprofile

import "context"

type Profile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system,omitempty"`

	Deinterlace       bool   `json:"deinterlace"`
	DeinterlaceMethod string `json:"deinterlace_method,omitempty"`

	RTSPProtocols string `json:"rtsp_protocols,omitempty"`
	RTSPLatency   int    `json:"rtsp_latency,omitempty"`

	HTTPTimeoutSec   int    `json:"http_timeout_sec,omitempty"`
	HTTPUserAgent    string `json:"http_user_agent,omitempty"`
	FormatHint       string `json:"format_hint,omitempty"`
	ProbeDurationSec int    `json:"probe_duration_sec,omitempty"`
}

type Store interface {
	Get(ctx context.Context, id string) (*Profile, error)
	List(ctx context.Context) ([]Profile, error)
	Create(ctx context.Context, p *Profile) error
	Update(ctx context.Context, p *Profile) error
	Delete(ctx context.Context, id string) error
}
