# WireGuard Plugin Interfaces

## Plugin (implements connectivity.Plugin)

```go
type Plugin struct { ... }

func New(transport http.RoundTripper) (*Plugin, error)
func (p *Plugin) Name() string
func (p *Plugin) ProxyURL(upstreamURL string) string
func (p *Plugin) HTTPClient() *http.Client
func (p *Plugin) IsConnected() bool
func (p *Plugin) Close() error
func (p *Plugin) Port() int
```

### New(transport)

Creates and starts the localhost proxy server. The `transport` parameter is the `http.RoundTripper` used for outbound requests from the proxy to upstream servers. Pass `nil` to use `http.DefaultTransport` (direct connectivity without WireGuard).

Returns an error if the listener cannot bind to a random port on localhost.

### Name() string

Returns `"wireguard"`. Used as the registry key.

### ProxyURL(upstreamURL string) string

Returns `http://127.0.0.1:{port}/?url={url.QueryEscape(upstreamURL)}`. The upstream URL is always URL-encoded.

### HTTPClient() *http.Client

Returns an `*http.Client` whose transport rewrites all requests to go through the localhost proxy. The original request URL becomes the `?url=` parameter.

### IsConnected() bool

Returns `true` when the proxy server is running. Returns `false` after `Close()` is called or if the server encounters a fatal error.

### Close() error

Shuts down the proxy server. Idempotent: calling Close multiple times is safe and returns nil.

### Port() int

Returns the randomly-assigned port the proxy is listening on.

## proxyTransport (internal)

```go
type proxyTransport struct {
    proxyBase string
}

func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error)
```

Implements `http.RoundTripper`. Rewrites requests so the original URL becomes a `?url=` query parameter on the proxy base URL. Used by `HTTPClient()` to route all traffic through the localhost proxy.

## Proxy Handler Behavior

The proxy HTTP handler:

1. Reads `?url=` query parameter (returns 400 if missing)
2. Creates a new request to the decoded upstream URL
3. Forwards `Range` headers from the original request
4. Proxies the response (status code, headers, body) back to the caller
5. Returns 502 on upstream connection failures or invalid URLs
