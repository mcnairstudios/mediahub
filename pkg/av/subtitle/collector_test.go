package subtitle

import (
	"strings"
	"testing"
)

func TestCollectorSRT(t *testing.T) {
	c := NewCollector(TrackInfo{Index: 0, Codec: "subrip", Language: "eng"})
	c.Push([]byte("<b>Hello</b> world"), 1_000_000_000, 2_000_000_000)

	vtt := string(c.WebVTT())
	if !strings.Contains(vtt, "WEBVTT") {
		t.Fatal("missing WEBVTT header")
	}
	if !strings.Contains(vtt, "Hello world") {
		t.Fatalf("expected stripped SRT text, got:\n%s", vtt)
	}
	if strings.Contains(vtt, "<b>") {
		t.Fatal("SRT tags should be stripped")
	}
}

func TestCollectorASS(t *testing.T) {
	c := NewCollector(TrackInfo{Index: 0, Codec: "ass", Language: "eng"})
	c.Push([]byte("{\\an8}Top text"), 1_000_000_000, 2_000_000_000)

	vtt := string(c.WebVTT())
	if !strings.Contains(vtt, "Top text") {
		t.Fatalf("expected stripped ASS text, got:\n%s", vtt)
	}
	if strings.Contains(vtt, "{\\an8}") {
		t.Fatal("ASS tags should be stripped")
	}
}

func TestCollectorEmptyText(t *testing.T) {
	c := NewCollector(TrackInfo{Index: 0, Codec: "subrip"})
	c.Push([]byte("  "), 0, 1_000_000_000)

	if c.CueCount() != 0 {
		t.Fatal("empty text should not add a cue")
	}
}

func TestCollectorReset(t *testing.T) {
	c := NewCollector(TrackInfo{Index: 0, Codec: "subrip"})
	c.Push([]byte("hello"), 0, 1_000_000_000)
	if c.CueCount() != 1 {
		t.Fatal("expected 1 cue before reset")
	}
	c.Reset()
	if c.CueCount() != 0 {
		t.Fatal("expected 0 cues after reset")
	}
}
