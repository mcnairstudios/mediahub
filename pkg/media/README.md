# pkg/media — Shared Media Types

## Purpose
The common vocabulary used across the entire system. Defines codecs, stream metadata, and probe results that both source and output packages reference.

## Responsibilities
- Define `VideoCodec`, `AudioCodec`, `Container` types with normalization
- Define the `Stream` struct — the unified representation of any media stream
- Define `ProbeResult`, `VideoInfo`, `AudioTrack` for stream analysis data
- Provide codec normalization (hevc→h265, aac_latm→aac, etc.)

## Does NOT
- Perform any I/O, network access, or media processing
- Depend on any other mediahub package — this is a leaf dependency
