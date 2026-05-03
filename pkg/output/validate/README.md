# validate

Manifest and segment validation for segment-based output plugins (HLS, MSE, DASH).

## Purpose

When a user reports broken playback, capture the manifest/segment data and write a test using these validators to reproduce the failure. The test stays in the suite forever as a regression guard.

## Validators

- **ValidateHLSPlaylist** - Checks m3u8 playlists for structural correctness (header, target duration, segment sequencing, duplicates, media sequence for live)
- **ValidateMPD** - Checks DASH MPD manifests for valid XML structure, namespace, Period/AdaptationSet/Representation hierarchy, SegmentTemplate attributes
- **ValidateFMP4Init** - Checks fMP4 init segments for required box hierarchy (ftyp, moov/trak/mdia/minf/stbl/stsd) and codec-specific boxes (avcC, hvcC, esds)
- **ValidateFMP4Segment** - Checks fMP4 media segments for moof/mdat structure, traf/tfhd/trun presence, non-zero sample count

## Usage

All validators are pure functions: bytes in, validation errors out. No IO, no HTTP.

```go
errs := validate.ValidateHLSPlaylist(playlistBytes)
if len(errs) > 0 {
    for _, e := range errs {
        t.Errorf("%s: %s", e.Field, e.Message)
    }
}
```

## Workflow

1. User reports broken playback
2. Capture the manifest/segment bytes
3. Write a test: `validate.ValidateHLSPlaylist(brokenData)` expecting specific errors
4. Fix the output plugin
5. Test passes and stays in the suite
