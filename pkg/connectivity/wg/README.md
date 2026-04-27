# WireGuard Connectivity Plugin

Routes HTTP traffic through a WireGuard tunnel via a localhost reverse proxy.

## Architecture

Three layers:

1. **Tunnel** (`tunnel.go`) - Userspace WireGuard tunnel using golang.zx2c4.com/wireguard with netstack TUN. Creates an in-process VPN that provides `DialContext` and `http.RoundTripper` for routing traffic through the tunnel.

2. **Plugin** (`wg.go`) - Localhost reverse proxy that implements `connectivity.Plugin`. Binds to `127.0.0.1:{random_port}` and forwards requests to upstream URLs through the tunnel's transport. This allows any component to route traffic through WireGuard by rewriting URLs to `http://127.0.0.1:{port}/?url=<encoded_upstream>`.

3. **Service** (`service.go`) - Profile management and tunnel lifecycle. CRUD for WireGuard profiles stored in SettingsStore, activation/deactivation of tunnels, health checks, and startup restoration.

## Usage

```go
// Create service with settings store
svc := wg.NewService(settingsStore)

// Restore previously active profile on startup
svc.RestoreActive(ctx)

// Create a profile
profile, err := svc.CreateProfile(ctx, wg.TunnelConfig{
    Name:       "My VPN",
    PrivateKey: "base64...",
    Endpoint:   "vpn.example.com:51820",
    PublicKey:  "base64...",
    Address:    "10.0.0.2/24",
    AllowedIPs: "0.0.0.0/0",
    DNS:        "1.1.1.1",
})

// Activate - creates tunnel + proxy
svc.Activate(ctx, profile.ID)

// Get the active plugin for connectivity registry
plugin := svc.ActivePlugin()
registry.Register(plugin)

// Check status
status := svc.Status()
// status.Connected, status.ProxyPort, status.Endpoint, etc.

// Test a profile without activating
result := svc.TestProfile(ctx, profile.ID)
// result.Success, result.LatencyMs, result.Error

// Cleanup
svc.Close()
```

## Transport Injection

The Plugin layer accepts any `http.RoundTripper`. When created via the Service, it receives the Tunnel's transport. For testing without a real VPN, pass `nil` to `New()` for direct connectivity.

## Storage

Profiles stored in SettingsStore with key pattern `wg_profile_{id}`. JSON-serialized. Private keys are stored unmasked but masked in all API responses (first 4 + last 4 chars visible).

## API

All endpoints require admin authentication.

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/wireguard/profiles | List profiles (private keys masked) |
| POST | /api/wireguard/profiles | Create profile |
| PUT | /api/wireguard/profiles/{id} | Update profile |
| DELETE | /api/wireguard/profiles/{id} | Delete profile + deactivate if active |
| POST | /api/wireguard/profiles/{id}/activate | Activate tunnel |
| POST | /api/wireguard/profiles/{id}/test | Health check via tunnel |
| GET | /api/wireguard/status | Current tunnel status |

## UI

WireGuard page (admin only) shows:
- Status bar: connected (green) or disconnected (yellow) with endpoint and proxy port
- Profile list with name, endpoint, address, active status
- Add/edit form for all profile fields
- Activate, Test, Edit, Delete buttons per profile
- Test shows latency on success, error message on failure
