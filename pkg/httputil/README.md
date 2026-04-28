# httputil

Shared HTTP utilities for mediahub handlers and services.

## Packages

### response.go

JSON response helpers for HTTP handlers.

- `RespondJSON(w, status, data)` — write JSON response with content-type header
- `RespondError(w, status, message)` — write JSON error response
- `DecodeJSON(r, v)` — decode request body JSON into a value

### headers.go

Browser-like headers and request URL helpers.

- `SetBrowserHeaders(req, userAgent)` — sets User-Agent, Accept, Accept-Language, Connection
- `RequestBaseURL(r)` — derive base URL from incoming request, respecting X-Forwarded-Proto and X-Forwarded-Host for reverse proxy setups

### fetch.go

Conditional HTTP fetch with ETag support.

- `FetchConditional(ctx, client, url, etag, userAgent, extraHeaders...)` — sends If-None-Match when etag provided, returns Changed=false on 304, Changed=true with body on 200. Optional `extraHeaders` maps are merged onto the request.

## Dependencies

Standard library only: net/http, encoding/json, io, context, fmt.
