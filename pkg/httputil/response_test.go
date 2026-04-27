package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"name": "test"}

	RespondJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["name"] != "test" {
		t.Fatalf("expected name=test, got %s", got["name"])
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()

	RespondError(w, http.StatusBadRequest, "bad input")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["error"] != "bad input" {
		t.Fatalf("expected error=bad input, got %s", got["error"])
	}
}

func TestDecodeJSON(t *testing.T) {
	body := strings.NewReader(`{"name":"alice","age":30}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)

	var got struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := DecodeJSON(req, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "alice" || got.Age != 30 {
		t.Fatalf("unexpected result: %+v", got)
	}
}
