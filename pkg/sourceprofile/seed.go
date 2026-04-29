package sourceprofile

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/mcnairstudios/mediahub/pkg/defaults"
)

func SeedDefaults(ctx context.Context, store Store, defs []defaults.SourceProfileDef) error {
	existing, err := store.List(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	for _, d := range defs {
		p := Profile{
			ID:                generateID(),
			Name:              d.Name,
			Deinterlace:       d.Deinterlace,
			DeinterlaceMethod: d.DeinterlaceMethod,
			AudioLanguage:     d.AudioLanguage,
			SubtitleLanguage:  d.SubtitleLanguage,
			RTSPProtocols:     d.RTSPProtocols,
			RTSPLatency:       d.RTSPLatency,
			HTTPTimeoutSec:    d.HTTPTimeoutSec,
			HTTPUserAgent:     d.HTTPUserAgent,
		}
		if err := store.Create(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
