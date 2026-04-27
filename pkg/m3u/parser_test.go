package m3u

import (
	"strings"
	"testing"
)

const testM3U = `#EXTM3U
#EXTINF:-1 tvg-id="bbc1" tvg-name="BBC One" tvg-logo="http://logo.png" group-title="UK",BBC One HD
http://stream.example.com/bbc1
#EXTINF:-1 tvg-id="itv1" group-title="UK",ITV
http://stream.example.com/itv1
`

func TestParseBasic(t *testing.T) {
	entries, err := Parse(strings.NewReader(testM3U))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	e := entries[0]
	if e.Name != "BBC One HD" {
		t.Errorf("expected name 'BBC One HD', got %q", e.Name)
	}
	if e.URL != "http://stream.example.com/bbc1" {
		t.Errorf("expected URL 'http://stream.example.com/bbc1', got %q", e.URL)
	}
	if e.TvgID != "bbc1" {
		t.Errorf("expected tvg-id 'bbc1', got %q", e.TvgID)
	}
	if e.TvgName != "BBC One" {
		t.Errorf("expected tvg-name 'BBC One', got %q", e.TvgName)
	}
	if e.TvgLogo != "http://logo.png" {
		t.Errorf("expected tvg-logo 'http://logo.png', got %q", e.TvgLogo)
	}
	if e.Group != "UK" {
		t.Errorf("expected group 'UK', got %q", e.Group)
	}
	if e.Duration != -1 {
		t.Errorf("expected duration -1, got %d", e.Duration)
	}

	e2 := entries[1]
	if e2.Name != "ITV" {
		t.Errorf("expected name 'ITV', got %q", e2.Name)
	}
	if e2.TvgName != "" {
		t.Errorf("expected empty tvg-name, got %q", e2.TvgName)
	}
	if e2.TvgLogo != "" {
		t.Errorf("expected empty tvg-logo, got %q", e2.TvgLogo)
	}
}

func TestParseMissingAttributes(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1,Channel Name
http://example.com/stream
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Name != "Channel Name" {
		t.Errorf("expected name 'Channel Name', got %q", e.Name)
	}
	if e.TvgID != "" {
		t.Errorf("expected empty tvg-id, got %q", e.TvgID)
	}
	if e.Group != "" {
		t.Errorf("expected empty group, got %q", e.Group)
	}
}

func TestParseNoHeader(t *testing.T) {
	input := `#EXTINF:-1 tvg-id="ch1",Channel 1
http://example.com/ch1
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].TvgID != "ch1" {
		t.Errorf("expected tvg-id 'ch1', got %q", entries[0].TvgID)
	}
}

func TestParseEmptyInput(t *testing.T) {
	entries, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseWhitespaceAndBlankLines(t *testing.T) {
	input := `#EXTM3U

  #EXTINF:-1 tvg-id="ch1" , Channel 1

  http://example.com/ch1

#EXTINF:-1,Channel 2
http://example.com/ch2

`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "Channel 1" {
		t.Errorf("expected name 'Channel 1', got %q", entries[0].Name)
	}
	if entries[0].URL != "http://example.com/ch1" {
		t.Errorf("expected URL 'http://example.com/ch1', got %q", entries[0].URL)
	}
}

func TestParseCustomAttributes(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 tvg-id="ch1" catchup="default" catchup-days="7" tvg-chno="42",Channel 1
http://example.com/ch1
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.TvgID != "ch1" {
		t.Errorf("expected tvg-id 'ch1', got %q", e.TvgID)
	}
	if e.Attributes["catchup"] != "default" {
		t.Errorf("expected catchup 'default', got %q", e.Attributes["catchup"])
	}
	if e.Attributes["catchup-days"] != "7" {
		t.Errorf("expected catchup-days '7', got %q", e.Attributes["catchup-days"])
	}
	if e.Attributes["tvg-chno"] != "42" {
		t.Errorf("expected tvg-chno '42', got %q", e.Attributes["tvg-chno"])
	}
}

func TestParseMultipleGroups(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 group-title="Sports",ESPN
http://example.com/espn
#EXTINF:-1 group-title="News",CNN
http://example.com/cnn
#EXTINF:-1 group-title="Sports",Fox Sports
http://example.com/fox
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Group != "Sports" {
		t.Errorf("expected group 'Sports', got %q", entries[0].Group)
	}
	if entries[1].Group != "News" {
		t.Errorf("expected group 'News', got %q", entries[1].Group)
	}
	if entries[2].Group != "Sports" {
		t.Errorf("expected group 'Sports', got %q", entries[2].Group)
	}
}

func TestParsePositiveDuration(t *testing.T) {
	input := `#EXTINF:300,Recorded Show
http://example.com/recorded
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Duration != 300 {
		t.Errorf("expected duration 300, got %d", entries[0].Duration)
	}
}

func TestParseMalformedLines(t *testing.T) {
	input := `#EXTM3U
not a valid line
#EXTINF:-1,Good Channel
http://example.com/good
#EXTINF:bad duration,Bad Duration
http://example.com/bad
just some random text
#EXTINF:-1,Another Good
http://example.com/another
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (skipping malformed), got %d", len(entries))
	}
	if entries[0].Name != "Good Channel" {
		t.Errorf("expected 'Good Channel', got %q", entries[0].Name)
	}
	if entries[1].Name != "Another Good" {
		t.Errorf("expected 'Another Good', got %q", entries[1].Name)
	}
}

func TestParseURLOnNextLine(t *testing.T) {
	input := `#EXTINF:-1 tvg-id="test",Test Channel
http://example.com/test
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].URL != "http://example.com/test" {
		t.Errorf("expected URL 'http://example.com/test', got %q", entries[0].URL)
	}
}

func TestParseOnlyHeader(t *testing.T) {
	entries, err := Parse(strings.NewReader("#EXTM3U\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseAttributeWithSpacesInValue(t *testing.T) {
	input := `#EXTINF:-1 tvg-name="BBC One HD" group-title="UK Freeview",BBC One HD
http://example.com/bbc1
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].TvgName != "BBC One HD" {
		t.Errorf("expected tvg-name 'BBC One HD', got %q", entries[0].TvgName)
	}
	if entries[0].Group != "UK Freeview" {
		t.Errorf("expected group 'UK Freeview', got %q", entries[0].Group)
	}
}
