package httputil

import (
	"fmt"
	"net/http"
)

func SetBrowserHeaders(req *http.Request, userAgent string) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
}

func RequestPort(r *http.Request) int {
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			port := 0
			for _, c := range host[i+1:] {
				if c >= '0' && c <= '9' {
					port = port*10 + int(c-'0')
				}
			}
			if port > 0 {
				return port
			}
		}
	}
	return 0
}

func RequestHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string, len(r.Header))
	for key := range r.Header {
		headers[key] = r.Header.Get(key)
	}
	return headers
}

func RequestBaseURL(r *http.Request) string {
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}
