# httputil -- Public API

No interfaces defined. This package provides shared HTTP utility functions.

## Functions

### RespondJSON

```go
func RespondJSON(w http.ResponseWriter, status int, data any)
```

Write a JSON response with the given status code.

### RespondError

```go
func RespondError(w http.ResponseWriter, status int, message string)
```

Write a JSON error response (`{"error": "..."}`) with the given status code.

### DecodeJSON

```go
func DecodeJSON(r *http.Request, v any) error
```

Decode the request body as JSON into v. Closes the body.

### FetchConditional

```go
func FetchConditional(ctx context.Context, client *http.Client, url, etag, userAgent string) (*FetchResult, error)
```

Fetch a URL with ETag-based conditional logic. Returns `Changed: false` on 304 Not Modified.

### SetBrowserHeaders

```go
func SetBrowserHeaders(req *http.Request, userAgent string)
```

Set User-Agent, Accept, Accept-Language, and Connection headers to mimic a browser.

### RequestBaseURL

```go
func RequestBaseURL(r *http.Request) string
```

Derive the base URL from the incoming request, respecting X-Forwarded-Proto and X-Forwarded-Host headers for reverse proxy setups.
