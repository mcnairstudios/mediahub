package httputil

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type FetchResult struct {
	Body    io.ReadCloser
	ETag    string
	Changed bool
}

func FetchConditional(ctx context.Context, client *http.Client, url, etag, userAgent string, extraHeaders ...map[string]string) (*FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	SetBrowserHeaders(req, userAgent)

	for _, headers := range extraHeaders {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}

	if resp.StatusCode == http.StatusNotModified {
		resp.Body.Close()
		return &FetchResult{
			ETag:    etag,
			Changed: false,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		logUpstreamFailure(resp, url)
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := DecompressReader(resp.Body, url)
	if err != nil {
		return nil, fmt.Errorf("decompressing response from %s: %w", url, err)
	}

	return &FetchResult{
		Body:    body,
		ETag:    resp.Header.Get("ETag"),
		Changed: true,
	}, nil
}

func logUpstreamFailure(resp *http.Response, url string) {
	var parts []string
	parts = append(parts, fmt.Sprintf("status=%d url=%s", resp.StatusCode, url))
	for _, name := range []string{"Server", "Cf-Ray", "Cf-Mitigated", "Content-Type", "Retry-After", "X-Cache"} {
		if v := resp.Header.Get(name); v != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", strings.ToLower(strings.ReplaceAll(name, "-", "_")), v))
		}
	}
	body := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, body)
	if n > 0 {
		parts = append(parts, fmt.Sprintf("body_snippet=%s", string(body[:n])))
	}
	log.Printf("httputil: upstream failure: %s", strings.Join(parts, " "))
}
