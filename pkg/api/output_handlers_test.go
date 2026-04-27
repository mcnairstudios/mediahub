package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/epg"
)

func TestOutputM3U(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	env.server.deps.ChannelStore.Create(ctx, &channel.Channel{
		ID:        "ch1",
		Name:      "BBC One",
		Number:    1,
		TvgID:     "bbc1.uk",
		LogoURL:   "http://example.com/bbc1.png",
		IsEnabled: true,
		StreamIDs: []string{"stream-1"},
	})
	env.server.deps.ChannelStore.Create(ctx, &channel.Channel{
		ID:        "ch2",
		Name:      "BBC Two",
		Number:    2,
		IsEnabled: true,
		StreamIDs: []string{"stream-2"},
	})
	env.server.deps.ChannelStore.Create(ctx, &channel.Channel{
		ID:        "ch3",
		Name:      "Disabled Channel",
		Number:    3,
		IsEnabled: false,
	})

	resp := env.request("GET", "/api/output/playlist.m3u", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "audio/x-mpegurl" {
		t.Errorf("Content-Type = %q, want %q", ct, "audio/x-mpegurl")
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	content := string(body)

	if !strings.HasPrefix(content, "#EXTM3U\n") {
		t.Error("M3U should start with #EXTM3U")
	}

	if !strings.Contains(content, `tvg-id="bbc1.uk"`) {
		t.Error("M3U should contain tvg-id for BBC One")
	}
	if !strings.Contains(content, `tvg-name="BBC One"`) {
		t.Error("M3U should contain tvg-name for BBC One")
	}
	if !strings.Contains(content, `tvg-logo="http://example.com/bbc1.png"`) {
		t.Error("M3U should contain tvg-logo for BBC One")
	}
	if !strings.Contains(content, "/channel/ch1") {
		t.Error("M3U should contain channel URL for ch1")
	}

	if !strings.Contains(content, `tvg-id="mediahub.ch2"`) {
		t.Error("M3U should use generated tvg-id for channels without TvgID")
	}

	if strings.Contains(content, "Disabled Channel") {
		t.Error("M3U should not contain disabled channels")
	}
}

func TestOutputM3UWithGroups(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()

	env.server.deps.GroupStore.Create(ctx, &channel.Group{
		ID:   "grp1",
		Name: "News",
	})

	env.server.deps.ChannelStore.Create(ctx, &channel.Channel{
		ID:        "ch1",
		Name:      "BBC News",
		Number:    1,
		GroupID:   "grp1",
		IsEnabled: true,
		StreamIDs: []string{"stream-1"},
	})

	resp := env.request("GET", "/api/output/playlist.m3u", nil, "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(body), `group-title="News"`) {
		t.Error("M3U should contain group-title for grouped channels")
	}
}

func TestOutputM3UEmpty(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/output/playlist.m3u", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != "#EXTM3U\n" {
		t.Errorf("empty M3U should be just header, got %q", string(body))
	}
}

func TestOutputEPG(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()

	env.server.deps.ChannelStore.Create(ctx, &channel.Channel{
		ID:        "ch1",
		Name:      "BBC One",
		Number:    1,
		TvgID:     "bbc1.uk",
		LogoURL:   "http://example.com/bbc1.png",
		IsEnabled: true,
	})

	now := time.Now()
	env.server.deps.ProgramStore.BulkInsert(ctx, []epg.Program{
		{
			ChannelID:   "bbc1.uk",
			Title:       "Test Programme",
			Description: "A test programme",
			StartTime:   now,
			EndTime:     now.Add(time.Hour),
			Rating:      "PG",
			IsNew:       true,
		},
	})

	resp := env.request("GET", "/api/output/epg.xml", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/xml" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/xml")
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	content := string(body)

	if !strings.Contains(content, `<?xml version="1.0"`) {
		t.Error("EPG should have XML declaration")
	}
	if !strings.Contains(content, `<tv generator-info-name="mediahub">`) {
		t.Error("EPG should have tv root element")
	}
	if !strings.Contains(content, `<channel id="bbc1.uk">`) {
		t.Error("EPG should contain channel element")
	}
	if !strings.Contains(content, `<display-name>BBC One</display-name>`) {
		t.Error("EPG should contain display-name")
	}
	if !strings.Contains(content, `<icon src="http://example.com/bbc1.png"`) {
		t.Error("EPG should contain icon")
	}
	if !strings.Contains(content, `<title>Test Programme</title>`) {
		t.Error("EPG should contain programme title")
	}
	if !strings.Contains(content, `<desc>A test programme</desc>`) {
		t.Error("EPG should contain programme description")
	}
	if !strings.Contains(content, `<new />`) {
		t.Error("EPG should contain new flag")
	}
	if !strings.Contains(content, `<value>PG</value>`) {
		t.Error("EPG should contain rating")
	}
}

func TestOutputEPGEmpty(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/output/epg.xml", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(body), `</tv>`) {
		t.Error("empty EPG should still have closing tv tag")
	}
}

func TestOutputEPGExcludesDisabledChannels(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	env.server.deps.ChannelStore.Create(ctx, &channel.Channel{
		ID:        "ch-disabled",
		Name:      "Disabled",
		Number:    1,
		TvgID:     "disabled.tv",
		IsEnabled: false,
	})

	env.server.deps.ProgramStore.BulkInsert(ctx, []epg.Program{
		{
			ChannelID: "disabled.tv",
			Title:     "Should Not Appear",
			StartTime: time.Now(),
			EndTime:   time.Now().Add(time.Hour),
		},
	})

	resp := env.request("GET", "/api/output/epg.xml", nil, "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if strings.Contains(string(body), "Should Not Appear") {
		t.Error("EPG should not contain programmes for disabled channels")
	}
}

func TestChannelStreamRedirect(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	env.server.deps.ChannelStore.Create(ctx, &channel.Channel{
		ID:        "ch1",
		Name:      "BBC One",
		Number:    1,
		IsEnabled: true,
		StreamIDs: []string{"stream-1"},
	})

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, _ := http.NewRequest("GET", env.httpServer.URL+"/channel/ch1", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "http://example.com/bbc1" {
		t.Errorf("Location = %q, want %q", location, "http://example.com/bbc1")
	}
}

func TestChannelStreamNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/channel/nonexistent", nil, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestChannelStreamNoStreams(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	env.server.deps.ChannelStore.Create(ctx, &channel.Channel{
		ID:        "empty-ch",
		Name:      "Empty",
		Number:    1,
		IsEnabled: true,
	})

	resp := env.request("GET", "/channel/empty-ch", nil, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestOutputM3UNoAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/output/playlist.m3u", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("output playlist should be unauthenticated, got %d", resp.StatusCode)
	}
}

func TestOutputEPGNoAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/output/epg.xml", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("output epg should be unauthenticated, got %d", resp.StatusCode)
	}
}

func TestXmlEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"<script>", "&lt;script&gt;"},
		{`He said "hi"`, `He said &quot;hi&quot;`},
		{"A & B", "A &amp; B"},
		{"it's", "it&apos;s"},
	}

	for _, tt := range tests {
		got := xmlEscape(tt.input)
		if got != tt.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
