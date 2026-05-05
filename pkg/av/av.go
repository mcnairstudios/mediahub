package av

import "math"

// NoPtsNanos is the nanosecond-domain sentinel for "PTS not set".
// Equivalent to ffmpeg's AV_NOPTS_VALUE but in our nanos domain.
const NoPtsNanos = math.MinInt64

type Packet struct {
	Type     StreamType
	Data     []byte
	PTS      int64 // nanoseconds (NoPtsNanos = unset)
	DTS      int64 // nanoseconds (NoPtsNanos = unset)
	Duration int64 // nanoseconds
	Keyframe bool
}

type StreamType int

const (
	Video StreamType = iota
	Audio
	Subtitle
)
