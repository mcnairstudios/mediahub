package keyenc

import (
	"testing"
)

type StreamKey struct {
	Kind       string `key:"streams"`
	SourceType string
	SourceID   string
	StreamID   string
}

type ChannelKey struct {
	Kind    string `key:"channels"`
	GroupID string
	ID      string
}

func TestKey(t *testing.T) {
	s := NewSchema[StreamKey]()
	k := s.Key(StreamKey{SourceType: "m3u", SourceID: "src-1", StreamID: "abc123"})
	if string(k) != "streams:m3u:src-1:abc123" {
		t.Errorf("got %q", k)
	}
}

func TestPrefix_Full(t *testing.T) {
	s := NewSchema[StreamKey]()
	p := s.Prefix(StreamKey{SourceType: "m3u", SourceID: "src-1"})
	if string(p) != "streams:m3u:src-1:" {
		t.Errorf("got %q", p)
	}
}

func TestPrefix_Partial(t *testing.T) {
	s := NewSchema[StreamKey]()
	p := s.Prefix(StreamKey{SourceType: "m3u"})
	if string(p) != "streams:m3u:" {
		t.Errorf("got %q", p)
	}
}

func TestPrefix_KindOnly(t *testing.T) {
	s := NewSchema[StreamKey]()
	p := s.Prefix(StreamKey{})
	if string(p) != "streams:" {
		t.Errorf("got %q", p)
	}
}

func TestParse(t *testing.T) {
	s := NewSchema[StreamKey]()
	v := s.Parse([]byte("streams:xtream:src-2:def456"))
	if v.SourceType != "xtream" || v.SourceID != "src-2" || v.StreamID != "def456" {
		t.Errorf("got %+v", v)
	}
}

func TestChannelKey(t *testing.T) {
	s := NewSchema[ChannelKey]()
	k := s.Key(ChannelKey{GroupID: "grp-1", ID: "ch-1"})
	if string(k) != "channels:grp-1:ch-1" {
		t.Errorf("got %q", k)
	}
	p := s.Prefix(ChannelKey{GroupID: "grp-1"})
	if string(p) != "channels:grp-1:" {
		t.Errorf("got %q", p)
	}
}

func TestReverse(t *testing.T) {
	k := Reverse("streamidx", "abc123")
	if string(k) != "streamidx:abc123" {
		t.Errorf("got %q", k)
	}
}
