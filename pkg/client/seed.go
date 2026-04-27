package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

func SeedDefaults(ctx context.Context, store Store) error {
	existing, err := store.List(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	defaults := []Client{
		{
			ID: generateClientID(), Name: "Browser", Priority: 100, ListenPort: 8080,
			IsEnabled: true, IsSystem: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
			},
			Profile: Profile{
				Delivery:   "mse",
				VideoCodec: "copy",
				AudioCodec: "aac",
				Container:  "mp4",
			},
		},
		{
			ID: generateClientID(), Name: "Jellyfin", Priority: 90, ListenPort: 8096,
			IsEnabled: true, IsSystem: true,
			Profile: Profile{
				Delivery:   "hls",
				VideoCodec: "copy",
				AudioCodec: "aac",
				Container:  "mpegts",
			},
		},
		{
			ID: generateClientID(), Name: "Plex", Priority: 80, ListenPort: 8080,
			IsEnabled: true, IsSystem: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Lavf/"},
			},
			Profile: Profile{
				Delivery:   "stream",
				VideoCodec: "copy",
				AudioCodec: "copy",
				Container:  "mpegts",
			},
		},
		{
			ID: generateClientID(), Name: "VLC", Priority: 70, ListenPort: 8080,
			IsEnabled: true, IsSystem: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "VLC/"},
			},
			Profile: Profile{
				Delivery:   "stream",
				VideoCodec: "copy",
				AudioCodec: "copy",
				Container:  "matroska",
			},
		},
		{
			ID: generateClientID(), Name: "LG TV", Priority: 60, ListenPort: 8080,
			IsEnabled: true, IsSystem: false,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Web0S"},
			},
			Profile: Profile{
				Delivery:   "stream",
				VideoCodec: "copy",
				AudioCodec: "copy",
				Container:  "mpegts",
			},
		},
		{
			ID: generateClientID(), Name: "Samsung TV", Priority: 55, ListenPort: 8080,
			IsEnabled: true, IsSystem: false,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "SMART-TV"},
			},
			Profile: Profile{
				Delivery:   "stream",
				VideoCodec: "copy",
				AudioCodec: "copy",
				Container:  "mpegts",
			},
		},
		{
			ID: generateClientID(), Name: "iPhone", Priority: 50, ListenPort: 8080,
			IsEnabled: true, IsSystem: false,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "iPhone"},
			},
			Profile: Profile{
				Delivery:   "hls",
				VideoCodec: "copy",
				AudioCodec: "aac",
				Container:  "mpegts",
			},
		},
		{
			ID: generateClientID(), Name: "HDHomeRun", Priority: 40, ListenPort: 8080,
			IsEnabled: false, IsSystem: false,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "HDHomeRun/"},
			},
			Profile: Profile{
				Delivery:   "stream",
				VideoCodec: "copy",
				AudioCodec: "copy",
				Container:  "mpegts",
			},
		},
	}

	for i := range defaults {
		if err := store.Create(ctx, &defaults[i]); err != nil {
			return err
		}
	}
	return nil
}

func generateClientID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
