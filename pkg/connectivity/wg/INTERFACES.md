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

## Tunnel

```go
type TunnelConfig struct {
    ID         string
    Name       string
    PrivateKey string
    Endpoint   string
    PublicKey  string
    AllowedIPs string
    DNS        string
    Address    string
    IsActive   bool
}

type PeerStats struct {
    TxBytes           int64
    RxBytes           int64
    LastHandshakeSec  int64
    LastHandshakeNsec int64
    Endpoint          string
}

type Tunnel struct { ... }

func NewTunnel(cfg TunnelConfig) (*Tunnel, error)
func (t *Tunnel) DialContext(ctx context.Context, network, address string) (net.Conn, error)
func (t *Tunnel) Transport() http.RoundTripper
func (t *Tunnel) HTTPClient(timeout time.Duration) *http.Client
func (t *Tunnel) Stats() (*PeerStats, error)
func (t *Tunnel) Config() TunnelConfig
func (t *Tunnel) Close()
```

### NewTunnel(cfg)

Creates a userspace WireGuard tunnel using golang.zx2c4.com/wireguard with netstack TUN. Resolves DNS endpoints, converts base64 keys to hex for IPC, brings the device up. Returns error if any step fails.

### DialContext

Routes TCP/UDP connections through the WireGuard tunnel. Used as the dialer for HTTP transports.

### Transport() http.RoundTripper

Returns an `http.Transport` configured with `DialContext` for the tunnel.

### HTTPClient(timeout)

Returns an `*http.Client` using the tunnel transport with the given timeout.

### Stats() (*PeerStats, error)

Queries the WireGuard device for peer statistics (tx/rx bytes, last handshake, endpoint).

### Close()

Closes the WireGuard device.

## Service

```go
type Service struct { ... }

func NewService(settings store.SettingsStore) *Service
func (s *Service) ListProfiles(ctx context.Context) ([]ProfileResponse, error)
func (s *Service) GetProfile(ctx context.Context, id string) (*ProfileResponse, error)
func (s *Service) CreateProfile(ctx context.Context, cfg TunnelConfig) (*ProfileResponse, error)
func (s *Service) UpdateProfile(ctx context.Context, id string, cfg TunnelConfig) (*ProfileResponse, error)
func (s *Service) DeleteProfile(ctx context.Context, id string) error
func (s *Service) Activate(ctx context.Context, id string) error
func (s *Service) Deactivate()
func (s *Service) TestProfile(ctx context.Context, id string) TestResult
func (s *Service) Status() StatusResponse
func (s *Service) ActivePlugin() *Plugin
func (s *Service) Close()
func (s *Service) RestoreActive(ctx context.Context) error
```

### Storage

Profiles stored in SettingsStore with key pattern `wg_profile_{id}`. JSON-serialized TunnelConfig. Private keys stored unmasked in the store but masked (first 4 + last 4 chars visible) in all API responses.

### Activate(ctx, id)

Deactivates any current tunnel, creates a new Tunnel + Plugin from the profile config, marks the profile as active in the store. Only one profile can be active at a time.

### TestProfile(ctx, id)

Creates a temporary tunnel from the profile config, makes an HTTP request to cloudflare.com/cdn-cgi/trace, measures latency. The tunnel is closed after the test regardless of outcome.

### RestoreActive(ctx)

Called on startup. Scans all profiles and activates the one marked as active (if any).

## Proxy Handler Behavior

The proxy HTTP handler:

1. Reads `?url=` query parameter (returns 400 if missing)
2. Creates a new request to the decoded upstream URL
3. Forwards `Range` headers from the original request
4. Proxies the response (status code, headers, body) back to the caller
5. Returns 502 on upstream connection failures or invalid URLs

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/wireguard/profiles | admin | List all WG profiles |
| POST | /api/wireguard/profiles | admin | Create profile |
| PUT | /api/wireguard/profiles/{id} | admin | Update profile |
| DELETE | /api/wireguard/profiles/{id} | admin | Delete profile |
| POST | /api/wireguard/profiles/{id}/activate | admin | Set as active tunnel |
| POST | /api/wireguard/profiles/{id}/test | admin | Health check |
| GET | /api/wireguard/status | admin | Current tunnel status |

## Utility

```go
func MaskPrivateKey(key string) string
```

Returns a masked version of the key with first 4 and last 4 characters visible, asterisks in between. Keys 8 chars or shorter return `"***"`.
