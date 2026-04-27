package xmltv

import (
	"strings"
	"testing"
	"time"
)

const testXMLTV = `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="bbc1.uk">
    <display-name>BBC One</display-name>
    <icon src="http://logo.png"/>
  </channel>
  <programme start="20260425180000 +0100" stop="20260425190000 +0100" channel="bbc1.uk">
    <title>EastEnders</title>
    <sub-title>Episode 1234</sub-title>
    <desc>Drama in Albert Square</desc>
    <category>Drama</category>
    <category>Soap</category>
    <rating><value>PG</value></rating>
    <episode-num system="onscreen">S01E05</episode-num>
  </programme>
</tv>`

func TestParseBasic(t *testing.T) {
	channels, programmes, err := Parse(strings.NewReader(testXMLTV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	ch := channels[0]
	if ch.ID != "bbc1.uk" {
		t.Errorf("channel ID = %q, want %q", ch.ID, "bbc1.uk")
	}
	if ch.DisplayName != "BBC One" {
		t.Errorf("channel DisplayName = %q, want %q", ch.DisplayName, "BBC One")
	}
	if ch.Icon != "http://logo.png" {
		t.Errorf("channel Icon = %q, want %q", ch.Icon, "http://logo.png")
	}

	if len(programmes) != 1 {
		t.Fatalf("expected 1 programme, got %d", len(programmes))
	}
	p := programmes[0]
	if p.ChannelID != "bbc1.uk" {
		t.Errorf("programme ChannelID = %q, want %q", p.ChannelID, "bbc1.uk")
	}
	if p.Title != "EastEnders" {
		t.Errorf("programme Title = %q, want %q", p.Title, "EastEnders")
	}
	if p.Subtitle != "Episode 1234" {
		t.Errorf("programme Subtitle = %q, want %q", p.Subtitle, "Episode 1234")
	}
	if p.Description != "Drama in Albert Square" {
		t.Errorf("programme Description = %q, want %q", p.Description, "Drama in Albert Square")
	}
}

func TestParseDatetime(t *testing.T) {
	channels, programmes, err := Parse(strings.NewReader(testXMLTV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = channels

	p := programmes[0]

	loc := time.FixedZone("+0100", 3600)
	wantStart := time.Date(2026, 4, 25, 18, 0, 0, 0, loc)
	wantStop := time.Date(2026, 4, 25, 19, 0, 0, 0, loc)

	if !p.Start.Equal(wantStart) {
		t.Errorf("Start = %v, want %v", p.Start, wantStart)
	}
	if !p.Stop.Equal(wantStop) {
		t.Errorf("Stop = %v, want %v", p.Stop, wantStop)
	}
}

func TestParseCategories(t *testing.T) {
	_, programmes, err := Parse(strings.NewReader(testXMLTV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := programmes[0]
	if len(p.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(p.Categories))
	}
	if p.Categories[0] != "Drama" {
		t.Errorf("categories[0] = %q, want %q", p.Categories[0], "Drama")
	}
	if p.Categories[1] != "Soap" {
		t.Errorf("categories[1] = %q, want %q", p.Categories[1], "Soap")
	}
}

func TestParseRating(t *testing.T) {
	_, programmes, err := Parse(strings.NewReader(testXMLTV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if programmes[0].Rating != "PG" {
		t.Errorf("Rating = %q, want %q", programmes[0].Rating, "PG")
	}
}

func TestParseEpisodeNum(t *testing.T) {
	_, programmes, err := Parse(strings.NewReader(testXMLTV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if programmes[0].EpisodeNum != "S01E05" {
		t.Errorf("EpisodeNum = %q, want %q", programmes[0].EpisodeNum, "S01E05")
	}
}

func TestParseMissingOptionalFields(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="test.ch">
    <display-name>Test</display-name>
  </channel>
  <programme start="20260425120000 +0000" stop="20260425130000 +0000" channel="test.ch">
    <title>Minimal</title>
  </programme>
</tv>`

	channels, programmes, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Icon != "" {
		t.Errorf("Icon = %q, want empty", channels[0].Icon)
	}

	if len(programmes) != 1 {
		t.Fatalf("expected 1 programme, got %d", len(programmes))
	}
	p := programmes[0]
	if p.Subtitle != "" {
		t.Errorf("Subtitle = %q, want empty", p.Subtitle)
	}
	if p.Description != "" {
		t.Errorf("Description = %q, want empty", p.Description)
	}
	if len(p.Categories) != 0 {
		t.Errorf("Categories len = %d, want 0", len(p.Categories))
	}
	if p.Rating != "" {
		t.Errorf("Rating = %q, want empty", p.Rating)
	}
	if p.EpisodeNum != "" {
		t.Errorf("EpisodeNum = %q, want empty", p.EpisodeNum)
	}
	if p.IsNew {
		t.Errorf("IsNew = true, want false (no episode-num)")
	}
}

func TestParseEmptyInput(t *testing.T) {
	channels, programmes, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(channels))
	}
	if len(programmes) != 0 {
		t.Errorf("expected 0 programmes, got %d", len(programmes))
	}
}

func TestParseEmptyTvElement(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?><tv></tv>`
	channels, programmes, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(channels))
	}
	if len(programmes) != 0 {
		t.Errorf("expected 0 programmes, got %d", len(programmes))
	}
}

func TestParseCredits(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <programme start="20260425180000 +0000" stop="20260425190000 +0000" channel="test.ch">
    <title>Movie</title>
    <credits>
      <director>Steven Spielberg</director>
      <director>Martin Scorsese</director>
      <actor>Tom Hanks</actor>
      <actor>Meryl Streep</actor>
      <actor>Denzel Washington</actor>
    </credits>
  </programme>
</tv>`

	_, programmes, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := programmes[0]
	if len(p.Credits.Directors) != 2 {
		t.Fatalf("expected 2 directors, got %d", len(p.Credits.Directors))
	}
	if p.Credits.Directors[0] != "Steven Spielberg" {
		t.Errorf("directors[0] = %q, want %q", p.Credits.Directors[0], "Steven Spielberg")
	}
	if p.Credits.Directors[1] != "Martin Scorsese" {
		t.Errorf("directors[1] = %q, want %q", p.Credits.Directors[1], "Martin Scorsese")
	}
	if len(p.Credits.Actors) != 3 {
		t.Fatalf("expected 3 actors, got %d", len(p.Credits.Actors))
	}
	if p.Credits.Actors[0] != "Tom Hanks" {
		t.Errorf("actors[0] = %q, want %q", p.Credits.Actors[0], "Tom Hanks")
	}
}

func TestParseIsNew(t *testing.T) {
	newProgramme := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <programme start="20260425180000 +0000" stop="20260425190000 +0000" channel="test.ch">
    <title>New Show</title>
    <episode-num system="onscreen">S01E01</episode-num>
  </programme>
</tv>`

	_, programmes, err := Parse(strings.NewReader(newProgramme))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !programmes[0].IsNew {
		t.Error("IsNew = false, want true (has episode-num, no previously-shown)")
	}

	rerunProgramme := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <programme start="20260425180000 +0000" stop="20260425190000 +0000" channel="test.ch">
    <title>Rerun</title>
    <episode-num system="onscreen">S01E01</episode-num>
    <previously-shown/>
  </programme>
</tv>`

	_, programmes, err = Parse(strings.NewReader(rerunProgramme))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if programmes[0].IsNew {
		t.Error("IsNew = true, want false (has previously-shown)")
	}
}

func TestParseDatetimeVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "positive offset",
			input: "20260425180000 +0100",
			want:  time.Date(2026, 4, 25, 18, 0, 0, 0, time.FixedZone("+0100", 3600)),
		},
		{
			name:  "negative offset",
			input: "20260425120000 -0500",
			want:  time.Date(2026, 4, 25, 12, 0, 0, 0, time.FixedZone("-0500", -18000)),
		},
		{
			name:  "UTC",
			input: "20260425000000 +0000",
			want:  time.Date(2026, 4, 25, 0, 0, 0, 0, time.FixedZone("+0000", 0)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseXMLTVTime(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMultipleChannelsAndProgrammes(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="bbc1.uk">
    <display-name>BBC One</display-name>
  </channel>
  <channel id="bbc2.uk">
    <display-name>BBC Two</display-name>
    <icon src="http://bbc2.png"/>
  </channel>
  <programme start="20260425180000 +0000" stop="20260425190000 +0000" channel="bbc1.uk">
    <title>Show A</title>
  </programme>
  <programme start="20260425190000 +0000" stop="20260425200000 +0000" channel="bbc1.uk">
    <title>Show B</title>
  </programme>
  <programme start="20260425180000 +0000" stop="20260425193000 +0000" channel="bbc2.uk">
    <title>Show C</title>
  </programme>
</tv>`

	channels, programmes, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
	if len(programmes) != 3 {
		t.Fatalf("expected 3 programmes, got %d", len(programmes))
	}
	if programmes[2].ChannelID != "bbc2.uk" {
		t.Errorf("programme[2] ChannelID = %q, want %q", programmes[2].ChannelID, "bbc2.uk")
	}
}
