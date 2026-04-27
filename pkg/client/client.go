package client

type Client struct {
	ID              string
	Name            string
	Priority        int
	ListenPort      int
	StreamProfileID string
	IsEnabled       bool
	MatchRules      []MatchRule
}

type MatchRule struct {
	HeaderName string
	MatchType  string
	MatchValue string
}

type Profile struct {
	Name         string
	Delivery     string
	VideoCodec   string
	AudioCodec   string
	Container    string
	HWAccel      string
	OutputHeight int
	Deinterlace  bool
}
