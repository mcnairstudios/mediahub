# pkg/av/extradata

Extracts and converts H.264/H.265 codec extradata between Annex B (raw NAL units with start codes) and ISO BMFF container format (avcC/hvcC). Also parses SPS NAL units via exp-golomb decoding to extract codec parameters (profile, level, chroma format, bit depth, resolution, interlace flag).

## Functions

- **ToCodecData** -- Converts Annex B extradata to ISO BMFF format (avcC for H.264, hvcC for H.265). Returns data unchanged if already in the correct format. Returns nil for unknown codecs or empty input.
- **ToHexString** -- Converts codec data bytes to hex string for use in container metadata.
- **SplitNALUnits** -- Splits an Annex B byte stream into individual NAL units, handling both 3-byte and 4-byte start codes.
- **ParseH264SPS** -- Parses an H.264 SPS NAL unit to extract profile, level, chroma format, bit depth, resolution, and interlace flag. Returns nil on parse failure.
- **ParseH265SPS** -- Parses an H.265 SPS NAL unit to extract chroma format, bit depth, and resolution. Returns nil on parse failure.

## Build

Pure Go, no CGO required:

```bash
go test ./pkg/av/extradata/...
```
