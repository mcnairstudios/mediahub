package sourceprofile

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

	defaults := []Profile{
		{
			ID:             generateID(),
			Name:           "Default",
			HTTPTimeoutSec: 30,
		},
		{
			ID:                generateID(),
			Name:              "SAT>IP DVB-T",
			Deinterlace:       true,
			DeinterlaceMethod: "auto",
			AudioLanguage:     "eng",
			RTSPProtocols:     "tcp",
			HTTPTimeoutSec:    10,
		},
		{
			ID:                generateID(),
			Name:              "DVB Satellite",
			Deinterlace:       true,
			DeinterlaceMethod: "auto",
			AudioLanguage:     "eng",
			RTSPProtocols:     "tcp",
			RTSPLatency:       200,
			HTTPTimeoutSec:    10,
		},
		{
			ID:             generateID(),
			Name:           "HDHomeRun",
			HTTPTimeoutSec: 10,
		},
		{
			ID:             generateID(),
			Name:           "Remote IPTV",
			HTTPTimeoutSec: 30,
		},
		{
			ID:             generateID(),
			Name:           "Local Network",
			HTTPTimeoutSec: 5,
		},
	}

	for i := range defaults {
		if err := store.Create(ctx, &defaults[i]); err != nil {
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
