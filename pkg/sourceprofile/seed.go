package sourceprofile

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/mcnairstudios/mediahub/pkg/defaults"
)

var renamedProfiles = map[string]string{
	"SAT>IP DVB-T": "SAT>IP",
	"DVB Satellite": "SAT>IP",
}

func SeedDefaults(ctx context.Context, store Store, defs []defaults.SourceProfileDef) error {
	existing, err := store.List(ctx)
	if err != nil {
		return err
	}

	names := make(map[string]bool)
	for _, e := range existing {
		names[e.Name] = true
	}

	for _, old := range existing {
		if newName, ok := renamedProfiles[old.Name]; ok {
			if !names[newName] {
				old.Name = newName
				store.Update(ctx, &old)
				names[newName] = true
			} else {
				store.Delete(ctx, old.ID)
			}
			delete(names, old.Name)
		}
	}

	for _, d := range defs {
		if names[d.Name] {
			continue
		}
		p := Profile{
			ID:                generateID(),
			Name:              d.Name,
			IsSystem:          true,
			Deinterlace:       d.Deinterlace,
			DeinterlaceMethod: d.DeinterlaceMethod,
			RTSPProtocols:     d.RTSPProtocols,
			RTSPLatency:       d.RTSPLatency,
			HTTPTimeoutSec:    d.HTTPTimeoutSec,
			HTTPUserAgent:     d.HTTPUserAgent,
			FormatHint:        d.FormatHint,
			ProbeDurationSec:  d.ProbeDurationSec,
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
