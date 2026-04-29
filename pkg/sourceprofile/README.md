# pkg/sourceprofile -- Source Profiles

## Purpose
Defines the data model and store interface for source profiles. A source profile controls how mediahub interacts with an input source: deinterlacing, language preferences, transport settings (RTSP, HTTP timeouts).

## Responsibilities
- Define the `Profile` struct (deinterlace, language, RTSP, HTTP settings)
- Define the `Store` interface for CRUD operations
- Provide `SeedDefaults` to populate initial profiles on first run

## Seed Defaults

Six profiles are seeded when the store is empty:

| Name | Key Settings |
|------|-------------|
| Default | 30s HTTP timeout |
| SAT>IP DVB-T | Deinterlace auto, eng audio, RTSP TCP, 10s timeout |
| DVB Satellite | Deinterlace auto, eng audio, RTSP TCP, 200ms latency, 10s timeout |
| HDHomeRun | 10s HTTP timeout |
| Remote IPTV | 30s HTTP timeout |
| Local Network | 5s HTTP timeout |

## Does NOT
- Persist data on disk -- bolt and other backends implement the Store interface externally
- Make codec or delivery decisions -- that is `pkg/strategy` and `pkg/client`
