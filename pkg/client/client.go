package client

import "context"

type Client struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Priority   int         `json:"priority"`
	ListenPort int         `json:"listen_port,omitempty"`
	IsEnabled  bool        `json:"is_enabled"`
	IsSystem   bool        `json:"is_system"`
	MatchRules []MatchRule `json:"match_rules,omitempty"`
	Profile    Profile     `json:"profile"`
}

type MatchRule struct {
	HeaderName string `json:"header_name"`
	MatchType  string `json:"match_type"`
	MatchValue string `json:"match_value"`
}

type Profile struct {
	Delivery     string `json:"delivery"`
	VideoCodec   string `json:"video_codec"`
	AudioCodec   string `json:"audio_codec"`
	Container    string `json:"container"`
	HWAccel      string `json:"hwaccel"`
	OutputHeight int    `json:"output_height,omitempty"`
}

type Store interface {
	Get(ctx context.Context, id string) (*Client, error)
	List(ctx context.Context) ([]Client, error)
	Create(ctx context.Context, c *Client) error
	Update(ctx context.Context, c *Client) error
	Delete(ctx context.Context, id string) error
}
