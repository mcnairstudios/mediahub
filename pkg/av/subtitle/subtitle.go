package subtitle

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

type Cue struct {
	StartMs int64
	EndMs   int64
	Text    string
}

type Writer struct {
	mu   sync.Mutex
	cues []Cue
}

func NewWriter() *Writer {
	return &Writer{}
}

func (w *Writer) AddCue(ptsNs, durationNs int64, text string) {
	startMs := ptsNs / 1_000_000
	endMs := (ptsNs + durationNs) / 1_000_000
	w.mu.Lock()
	w.cues = append(w.cues, Cue{
		StartMs: startMs,
		EndMs:   endMs,
		Text:    text,
	})
	w.mu.Unlock()
}

func (w *Writer) CueCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.cues)
}

func (w *Writer) WriteTo(out io.Writer) (int64, error) {
	w.mu.Lock()
	cues := make([]Cue, len(w.cues))
	copy(cues, w.cues)
	w.mu.Unlock()

	var buf strings.Builder
	buf.WriteString("WEBVTT\n")
	for _, c := range cues {
		buf.WriteString("\n")
		buf.WriteString(FormatTimestamp(c.StartMs))
		buf.WriteString(" --> ")
		buf.WriteString(FormatTimestamp(c.EndMs))
		buf.WriteString("\n")
		buf.WriteString(c.Text)
		buf.WriteString("\n")
	}
	n, err := io.WriteString(out, buf.String())
	return int64(n), err
}

var srtTagRe = regexp.MustCompile(`<[^>]+>`)

var assPositionInSRTRe = regexp.MustCompile(`\{\\[^}]*\}`)

func ConvertSRT(srt string) string {
	result := assPositionInSRTRe.ReplaceAllString(srt, "")
	result = srtTagRe.ReplaceAllString(result, "")
	return result
}

var assOverrideRe = regexp.MustCompile(`\{\\[^}]*\}`)

func ConvertASS(ass string) string {
	return assOverrideRe.ReplaceAllString(ass, "")
}

func FormatTimestamp(ms int64) string {
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	s := (ms % 60000) / 1000
	ms = ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
