package av

type Packet struct {
	Type     StreamType
	Data     []byte
	PTS      int64 // nanoseconds
	DTS      int64 // nanoseconds
	Duration int64 // nanoseconds
	Keyframe bool
}

type StreamType int

const (
	Video StreamType = iota
	Audio
	Subtitle
)
