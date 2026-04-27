package httputil

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchConditional200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	result, err := FetchConditional(context.Background(), srv.Client(), srv.URL, "", "TestAgent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	if !result.Changed {
		t.Fatal("expected Changed=true")
	}
	if result.ETag != `"abc123"` {
		t.Fatalf("expected ETag abc123, got %s", result.ETag)
	}

	body, _ := io.ReadAll(result.Body)
	if string(body) != "hello" {
		t.Fatalf("expected body hello, got %s", body)
	}
}

func TestFetchConditional304(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"abc123"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	result, err := FetchConditional(context.Background(), srv.Client(), srv.URL, `"abc123"`, "TestAgent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Changed {
		t.Fatal("expected Changed=false")
	}
	if result.Body != nil {
		t.Fatal("expected nil body on 304")
	}
}

func TestFetchConditionalSendsIfNoneMatch(t *testing.T) {
	var receivedETag string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedETag = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	FetchConditional(context.Background(), srv.Client(), srv.URL, `"etag-value"`, "TestAgent")

	if receivedETag != `"etag-value"` {
		t.Fatalf("expected If-None-Match etag-value, got %s", receivedETag)
	}
}
