package hls

// Rendition describes a single ABR variant with resolution and bitrate.
type Rendition struct {
	// Height is the output video height in pixels. Width is computed from
	// the source aspect ratio, rounded to an even number.
	Height int

	// Bitrate is the target video bitrate in bits/sec for the BANDWIDTH tag
	// in the master playlist. It is also passed to the encoder.
	Bitrate int
}

// DefaultRenditions returns a standard ABR rendition ladder.
// The caller should filter out any renditions whose Height exceeds the
// source resolution.
func DefaultRenditions() []Rendition {
	return []Rendition{
		{Height: 1080, Bitrate: 5_000_000},
		{Height: 720, Bitrate: 3_000_000},
		{Height: 480, Bitrate: 1_500_000},
		{Height: 360, Bitrate: 800_000},
	}
}
