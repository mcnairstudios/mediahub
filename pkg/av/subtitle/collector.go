package subtitle

import (
	"bytes"
	"strings"
	"sync"
)

type TrackInfo struct {
	Index    int
	Codec    string
	Language string
}

type Collector struct {
	mu     sync.RWMutex
	writer *Writer
	track  TrackInfo
}

func NewCollector(track TrackInfo) *Collector {
	return &Collector{
		writer: NewWriter(),
		track:  track,
	}
}

func (c *Collector) Push(data []byte, ptsNs int64, durationNs int64) {
	raw := string(data)
	var text string
	switch {
	case strings.Contains(c.track.Codec, "ass") || strings.Contains(c.track.Codec, "ssa"):
		text = ConvertASS(raw)
	case strings.Contains(c.track.Codec, "subrip") || strings.Contains(c.track.Codec, "srt"):
		text = ConvertSRT(raw)
	default:
		text = raw
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	c.writer.AddCue(ptsNs, durationNs, text)
}

func (c *Collector) WebVTT() []byte {
	var buf bytes.Buffer
	c.writer.WriteTo(&buf)
	return buf.Bytes()
}

func (c *Collector) CueCount() int {
	return c.writer.CueCount()
}

func (c *Collector) Track() TrackInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.track
}

func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writer = NewWriter()
}
