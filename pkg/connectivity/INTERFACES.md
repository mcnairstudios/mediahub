# Connectivity Plugin Interfaces

## Core Interface

```go
type Plugin interface {
    // Name identifies this connectivity plugin (e.g. "wireguard", "tailscale")
    Name() string

    // ProxyURL rewrites an upstream URL to route through this plugin's tunnel.
    // Returns the original URL unchanged if tunneling is not needed.
    ProxyURL(upstreamURL string) string

    // HTTPClient returns an *http.Client configured to route through this tunnel.
    // Used for non-streaming HTTP requests (M3U fetch, EPG fetch, etc.)
    HTTPClient() *http.Client

    // IsConnected reports whether the tunnel is currently active.
    IsConnected() bool

    // Close tears down the tunnel.
    Close() error
}
```

## Registry

```go
type Registry struct { ... }

func NewRegistry() *Registry
func (r *Registry) Register(p Plugin)
func (r *Registry) Get(name string) (Plugin, bool)
func (r *Registry) Active() Plugin  // returns the currently active plugin, or nil
func (r *Registry) SetActive(name string) error
func (r *Registry) List() []string
```

## Usage Pattern

```
Source needs to fetch upstream:
  1. Check source.UsesVPN()
  2. If true, get active connectivity plugin from registry
  3. For streaming: rewrite URL via plugin.ProxyURL(url)
  4. For HTTP: use plugin.HTTPClient()
  5. If false or no plugin: use direct connection
```
