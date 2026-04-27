# WireGuard Connectivity Plugin

Routes HTTP traffic through a WireGuard tunnel via a localhost reverse proxy.

## How It Works

1. `New(transport)` starts an HTTP server on `127.0.0.1:{random_port}`
2. Requests to `/?url=<encoded_upstream>` are reverse-proxied to the decoded URL
3. The reverse proxy uses the provided `http.RoundTripper` for outbound requests
4. Range headers are forwarded for seekable media streams

## Usage

```go
// With default transport (no actual WG tunnel — useful for testing/direct proxy)
plugin, err := wg.New(nil)

// With a real WireGuard transport
plugin, err := wg.New(wgTransport)

// Get proxy URL for a stream
proxyURL := plugin.ProxyURL("http://iptv.example.com/stream.ts")
// => "http://127.0.0.1:54321/?url=http%3A%2F%2Fiptv.example.com%2Fstream.ts"

// Get an HTTP client that routes through the proxy
client := plugin.HTTPClient()

// Register with the connectivity registry
registry := connectivity.NewRegistry()
registry.Register(plugin)

// Cleanup
plugin.Close()
```

## Transport Injection

The `transport` parameter to `New()` controls how outbound requests from the proxy reach the internet. Pass `nil` for direct connectivity, or inject a WireGuard-configured `http.RoundTripper` for tunnel routing.

## Plugin Interface

Implements `connectivity.Plugin`:

- `Name()` — returns `"wireguard"`
- `ProxyURL(upstream)` — returns localhost proxy URL with encoded upstream
- `HTTPClient()` — returns client configured to route through the proxy
- `IsConnected()` — true when the proxy server is running
- `Close()` — shuts down the proxy server (idempotent)
