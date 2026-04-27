package client

import "testing"

func TestDetect_BrowserByUserAgent(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "browser", Name: "Browser", Priority: 1, IsEnabled: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
			},
		},
	})

	got := d.Detect(8080, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	})

	if got == nil {
		t.Fatal("expected browser client, got nil")
	}
	if got.ID != "browser" {
		t.Fatalf("expected browser, got %s", got.ID)
	}
}

func TestDetect_PlexByUserAgent(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "plex", Name: "Plex", Priority: 1, IsEnabled: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "PMS/"},
			},
		},
	})

	got := d.Detect(8080, map[string]string{
		"User-Agent": "PlexMediaServer PMS/1.40.0",
	})

	if got == nil {
		t.Fatal("expected plex client, got nil")
	}
	if got.ID != "plex" {
		t.Fatalf("expected plex, got %s", got.ID)
	}
}

func TestDetect_HDHomeRunByUserAgent(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "hdhr", Name: "HDHomeRun", Priority: 1, IsEnabled: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "HDHomeRun/"},
			},
		},
	})

	got := d.Detect(8080, map[string]string{
		"User-Agent": "HDHomeRun/20230501 CFNetwork/1485",
	})

	if got == nil {
		t.Fatal("expected hdhr client, got nil")
	}
	if got.ID != "hdhr" {
		t.Fatalf("expected hdhr, got %s", got.ID)
	}
}

func TestDetect_JellyfinByPort(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "jellyfin", Name: "Jellyfin", Priority: 10, ListenPort: 8096,
			IsEnabled: true,
		},
	})

	got := d.Detect(8096, map[string]string{
		"User-Agent": "anything",
	})

	if got == nil {
		t.Fatal("expected jellyfin client, got nil")
	}
	if got.ID != "jellyfin" {
		t.Fatalf("expected jellyfin, got %s", got.ID)
	}
}

func TestDetect_JellyfinPortMismatch(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "jellyfin", Name: "Jellyfin", Priority: 10, ListenPort: 8096,
			IsEnabled: true,
		},
	})

	got := d.Detect(8080, map[string]string{
		"User-Agent": "anything",
	})

	if got != nil {
		t.Fatalf("expected nil for wrong port, got %s", got.ID)
	}
}

func TestDetect_HigherPriorityWins(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "low", Name: "Low", Priority: 1, IsEnabled: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
			},
		},
		{
			ID: "high", Name: "High", Priority: 10, IsEnabled: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
			},
		},
	})

	got := d.Detect(8080, map[string]string{
		"User-Agent": "Mozilla/5.0",
	})

	if got == nil {
		t.Fatal("expected high priority client, got nil")
	}
	if got.ID != "high" {
		t.Fatalf("expected high, got %s", got.ID)
	}
}

func TestDetect_NoMatch(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "plex", Name: "Plex", Priority: 1, IsEnabled: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "PMS/"},
			},
		},
	})

	got := d.Detect(8080, map[string]string{
		"User-Agent": "curl/7.88.1",
	})

	if got != nil {
		t.Fatalf("expected nil, got %s", got.ID)
	}
}

func TestDetect_DisabledClientSkipped(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "disabled", Name: "Disabled", Priority: 10, IsEnabled: false,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
			},
		},
		{
			ID: "enabled", Name: "Enabled", Priority: 1, IsEnabled: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
			},
		},
	})

	got := d.Detect(8080, map[string]string{
		"User-Agent": "Mozilla/5.0",
	})

	if got == nil {
		t.Fatal("expected enabled client, got nil")
	}
	if got.ID != "enabled" {
		t.Fatalf("expected enabled, got %s", got.ID)
	}
}

func TestDetect_ANDLogicAllRulesMustMatch(t *testing.T) {
	d := NewDetector([]Client{
		{
			ID: "specific", Name: "Specific", Priority: 1, IsEnabled: true,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
				{HeaderName: "X-Custom", MatchType: "exact", MatchValue: "special"},
			},
		},
	})

	got := d.Detect(8080, map[string]string{
		"User-Agent": "Mozilla/5.0",
	})
	if got != nil {
		t.Fatal("expected nil when second rule missing, got match")
	}

	got = d.Detect(8080, map[string]string{
		"User-Agent": "Mozilla/5.0",
		"X-Custom":   "special",
	})
	if got == nil {
		t.Fatal("expected match when both rules satisfy, got nil")
	}
	if got.ID != "specific" {
		t.Fatalf("expected specific, got %s", got.ID)
	}
}

func TestMatch_ContainsType(t *testing.T) {
	c := Client{
		ID: "test", IsEnabled: true,
		MatchRules: []MatchRule{
			{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Chrome"},
		},
	}

	if !Match(c, 0, map[string]string{"User-Agent": "Mozilla/5.0 Chrome/131.0"}) {
		t.Fatal("expected contains match")
	}
	if Match(c, 0, map[string]string{"User-Agent": "Mozilla/5.0 Firefox/120"}) {
		t.Fatal("expected no match for Firefox")
	}
}

func TestMatch_PrefixType(t *testing.T) {
	c := Client{
		ID: "test", IsEnabled: true,
		MatchRules: []MatchRule{
			{HeaderName: "User-Agent", MatchType: "prefix", MatchValue: "Dalvik/"},
		},
	}

	if !Match(c, 0, map[string]string{"User-Agent": "Dalvik/2.1.0 (Linux; Android 14)"}) {
		t.Fatal("expected prefix match")
	}
	if Match(c, 0, map[string]string{"User-Agent": "Something Dalvik/2.1.0"}) {
		t.Fatal("expected no match when prefix is wrong")
	}
}

func TestMatch_ExactType(t *testing.T) {
	c := Client{
		ID: "test", IsEnabled: true,
		MatchRules: []MatchRule{
			{HeaderName: "X-Client-ID", MatchType: "exact", MatchValue: "tvproxy-special"},
		},
	}

	if !Match(c, 0, map[string]string{"X-Client-ID": "tvproxy-special"}) {
		t.Fatal("expected exact match")
	}
	if Match(c, 0, map[string]string{"X-Client-ID": "tvproxy-special-v2"}) {
		t.Fatal("expected no match for non-exact value")
	}
}

func TestMatch_RegexType(t *testing.T) {
	c := Client{
		ID: "test", IsEnabled: true,
		MatchRules: []MatchRule{
			{HeaderName: "User-Agent", MatchType: "regex", MatchValue: `Web0S|webOS`},
		},
	}

	if !Match(c, 0, map[string]string{"User-Agent": "Web0S SmartTV"}) {
		t.Fatal("expected regex match for Web0S")
	}
	if !Match(c, 0, map[string]string{"User-Agent": "webOS/6.0"}) {
		t.Fatal("expected regex match for webOS")
	}
	if Match(c, 0, map[string]string{"User-Agent": "Mozilla/5.0"}) {
		t.Fatal("expected no regex match")
	}
}

func TestMatch_PortZeroMatchesAny(t *testing.T) {
	c := Client{
		ID: "test", IsEnabled: true, ListenPort: 0,
		MatchRules: []MatchRule{
			{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "test"},
		},
	}

	if !Match(c, 8080, map[string]string{"User-Agent": "test"}) {
		t.Fatal("ListenPort 0 should match any port")
	}
	if !Match(c, 8096, map[string]string{"User-Agent": "test"}) {
		t.Fatal("ListenPort 0 should match any port")
	}
}

func TestMatch_NoRulesPortOnly(t *testing.T) {
	c := Client{
		ID: "jf", IsEnabled: true, ListenPort: 8096,
	}

	if !Match(c, 8096, map[string]string{}) {
		t.Fatal("port-only client with matching port should match")
	}
	if Match(c, 8080, map[string]string{}) {
		t.Fatal("port-only client with wrong port should not match")
	}
}
