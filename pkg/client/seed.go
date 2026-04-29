package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/mcnairstudios/mediahub/pkg/defaults"
)

func SeedDefaults(ctx context.Context, store Store, defs []defaults.ClientDef) error {
	existing, err := store.List(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	for _, d := range defs {
		c := Client{
			ID:         generateClientID(),
			Name:       d.Name,
			Priority:   d.Priority,
			ListenPort: d.ListenPort,
			IsEnabled:  d.IsEnabled,
			IsSystem:   d.IsSystem,
			Profile: Profile{
				Delivery:     d.Profile.Delivery,
				VideoCodec:   d.Profile.VideoCodec,
				AudioCodec:   d.Profile.AudioCodec,
				Container:    d.Profile.Container,
				HWAccel:      d.Profile.HWAccel,
				OutputHeight: d.Profile.OutputHeight,
			},
		}
		for _, r := range d.MatchRules {
			c.MatchRules = append(c.MatchRules, MatchRule{
				HeaderName: r.HeaderName,
				MatchType:  r.MatchType,
				MatchValue: r.MatchValue,
			})
		}
		if err := store.Create(ctx, &c); err != nil {
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
