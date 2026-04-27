# pkg/frontend/dlna

DLNA/UPnP MediaServer frontend for mediahub. Advertises on the local network via SSDP so that TVs (LG, Samsung, Panasonic), Oculus Quest, and other DLNA clients can browse channels and play streams.

## Components

### Server (server.go)

HTTP handler for UPnP device description and SOAP control endpoints.

Routes:
- `GET /dlna/device.xml` -- UPnP device description
- `GET /dlna/ContentDirectory.xml` -- content directory service description
- `GET /dlna/ConnectionManager.xml` -- connection manager service description
- `POST /dlna/control/ContentDirectory` -- SOAP Browse, GetSearchCapabilities, GetSortCapabilities, GetSystemUpdateID
- `POST /dlna/control/ConnectionManager` -- SOAP GetProtocolInfo, GetCurrentConnectionIDs, GetCurrentConnectionInfo

Browse hierarchy:
- Root (object ID "0") -- lists channel groups + "Ungrouped" container
- Group (object ID "grp-{id}") -- lists channels in that group with pagination
- Channel (object ID "ch-{id}") -- channel metadata with stream URL

### SSDP Advertiser (ssdp.go)

Multicast advertisement and M-SEARCH response handler.

- Sends `ssdp:alive` notifications on 239.255.255.250:1900 at configurable intervals
- Responds to M-SEARCH for `ssdp:all`, `upnp:rootdevice`, and `MediaServer:1`
- Sends `ssdp:byebye` on shutdown
- Respects `SettingsChecker.IsEnabled()` -- skips advertisement when disabled

### Types (types.go)

SOAP envelope/body types, Browse request struct, channel/group items for the `ChannelLister` interface.

## Usage

```go
channels := &myChannelAdapter{store: channelStore, groupStore: groupStore}
settings := &mySettingsAdapter{store: settingsStore}
log := zerolog.New(os.Stdout)

srv := dlna.NewServer(channels, settings, "http://192.168.1.100", 8080, log)
srv.RegisterRoutes(mux)

adv := dlna.NewSSDPAdvertiser(srv, "http://192.168.1.100", 8080, 30*time.Second, log)
go adv.Run(ctx)
```

## Testing

```bash
go test ./pkg/frontend/dlna/...
```
