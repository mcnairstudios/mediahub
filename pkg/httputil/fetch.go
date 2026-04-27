package httputil

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	return &FetchResult{
		Body:    resp.Body,
		ETag:    resp.Header.Get("ETag"),
		Changed: true,
	}, nil
}
