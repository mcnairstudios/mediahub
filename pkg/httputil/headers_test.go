package httputil

import (
	"net/http"
	"testing"
)

func TestSetBrowserHeadersUserAgent(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	SetBrowserHeaders(req, "TestAgent/1.0")

	if ua := req.Header.Get("User-Agent"); ua != "TestAgent/1.0" {
		t.Fatalf("expected User-Agent TestAgent/1.0, got %s", ua)
	}
}

func TestSetBrowserHeadersAccept(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	SetBrowserHeaders(req, "TestAgent/1.0")

	accept := req.Header.Get("Accept")
	if accept == "" {
		t.Fatal("expected Accept header to be set")
	}
}
