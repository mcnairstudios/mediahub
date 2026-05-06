# Source Plugin Development Guide

This guide covers how to create a source plugin for MediaHub. A source plugin provides streams (live TV channels, VOD content, radio stations, etc.) from an upstream provider.

## What is a Source Plugin

A source plugin connects MediaHub to an external stream provider. It implements the `Source` interface to:

- Fetch and refresh a list of streams from the provider
- Store streams via the `StreamStore` dependency
- Report its status (stream count, last refresh, errors)

Built-in examples: M3U, Xtream Codes, HDHomeRun, SAT>IP, Demo, SpaceX, Radio Garden.

## Plugin Package Structure

A plugin lives in its own package under `pkg/source/`:

```
pkg/source/myplugin/
    myplugin.go       # Source implementation + Config struct
    myplugin_test.go  # Tests
```

Each plugin package must:

1. Define a `Config` struct with its dependencies
2. Define a `Source` struct embedding `source.BaseSource`
3. Implement the `source.Source` interface
4. Provide a `New(cfg Config) *Source` constructor

## ConfigField Types

Config fields define the UI form rendered when a user adds or edits a source. Each field maps to a specific input type.

| FieldType       | Constant          | Renders As                        |
|-----------------|-------------------|-----------------------------------|
| `"text"`        | `FieldText`       | Single-line text input            |
| `"password"`    | `FieldPassword`   | Masked password input             |
| `"url"`         | `FieldURL`        | URL input with validation         |
| `"number"`      | `FieldNumber`     | Numeric input                     |
| `"bool"`        | `FieldBool`       | Toggle/checkbox                   |
| `"select"`      | `FieldSelect`     | Dropdown with `Options` slice     |
| `"hidden"`      | `FieldHidden`     | Hidden field (not shown in UI)    |
| `"custom"`      | `FieldCustom`     | Custom UI component via `Component` field |

### ConfigField struct

```go
type ConfigField struct {
    Key         string    `json:"key"`          // Config map key
    Label       string    `json:"label"`        // Display label
    Type        FieldType `json:"type"`         // Input type
    Required    bool      `json:"required"`     // Validation
    Default     string    `json:"default"`      // Default value
    Placeholder string    `json:"placeholder"`  // Placeholder text
    HelpText    string    `json:"help_text"`    // Help text below input
    Options     []Option  `json:"options"`      // For FieldSelect
    Component   string    `json:"component"`    // For FieldCustom
}
```

## Custom UI Components

For complex inputs (device pickers, map selectors), use `FieldCustom` with a `Component` name and provide JavaScript via `FrontendJS`:

```go
source.PluginRegistration{
    Descriptor: source.PluginDescriptor{
        // ...
        ConfigFields: []source.ConfigField{
            {
                Key:       "devices",
                Label:     "Devices",
                Type:      source.FieldCustom,
                Component: "my-device-picker",
                HelpText:  "Select devices to import",
            },
        },
    },
    FrontendJS: []byte(`
        // JavaScript that registers the "my-device-picker" component.
        // The frontend loads this and renders it when Component matches.
    `),
}
```

The `FrontendJS` bytes are served to the browser and executed. The JavaScript must register a component matching the `Component` field name.

## How to Register a Plugin

### Option A: Self-registering via init()

The plugin registers itself when imported. This is the recommended approach for external/third-party plugins.

```go
package myplugin

import "github.com/mcnairstudios/mediahub/pkg/source"

func init() {
    source.DefaultRegistry.RegisterPlugin(source.PluginRegistration{
        Descriptor: source.PluginDescriptor{
            Type:       "myplugin",
            Label:      "My Plugin",
            ShortLabel: "MINE",
            Color:      "#2196f3",
            Version:    "1.0.0",
        },
        Factory: func(ctx context.Context, sourceID string) (source.Source, error) {
            // Create and return source instance
        },
    })
}
```

Then import it with a blank import in `cmd/mediahub/main.go`:

```go
import _ "github.com/mcnairstudios/mediahub/pkg/source/myplugin"
```

### Option B: Explicit registration in sources.go

Built-in plugins register their factory via `reg.Register()` in `cmd/mediahub/sources.go`, and descriptors are added in `registerBuiltinDescriptors()`. This approach is used for plugins that need injected dependencies (stores, config, etc.).

## Installation Flow (External Plugin)

1. `go get github.com/example/mediahub-myplugin`
2. Add blank import: `import _ "github.com/example/mediahub-myplugin"`
3. Rebuild: `make build`

The `init()` function registers the plugin with `DefaultRegistry`, making it available in the UI and API automatically.

## Complete Minimal Example

A source with no config fields (like Demo or SpaceX):

```go
package myplugin

import (
    "context"
    "crypto/sha256"
    "fmt"

    "github.com/mcnairstudios/mediahub/pkg/media"
    "github.com/mcnairstudios/mediahub/pkg/source"
    "github.com/mcnairstudios/mediahub/pkg/store"
)

const PluginType source.SourceType = "myplugin"

type Config struct {
    ID            string
    Name          string
    IsEnabled     bool
    StreamStore   store.StreamStore
    OnRefreshDone func(sourceID, etag string, streamCount int)
}

type Source struct {
    source.BaseSource
    cfg Config
}

func New(cfg Config) *Source {
    return &Source{
        BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, PluginType, cfg.IsEnabled, 0),
        cfg:        cfg,
    }
}

func (s *Source) Refresh(ctx context.Context) error {
    streams := []media.Stream{
        {
            ID:         deterministicID(s.cfg.ID, "stream-1"),
            SourceType: string(PluginType),
            SourceID:   s.cfg.ID,
            Name:       "My Stream",
            URL:        "https://example.com/stream.m3u8",
            Group:      "My Plugin",
            IsActive:   true,
        },
    }

    keepIDs := make([]string, len(streams))
    for i, st := range streams {
        keepIDs[i] = st.ID
    }

    if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
        s.SetError(err.Error())
        return fmt.Errorf("upserting streams: %w", err)
    }

    if _, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, string(PluginType), s.cfg.ID, keepIDs); err != nil {
        s.SetError(err.Error())
        return fmt.Errorf("deleting stale streams: %w", err)
    }

    s.SetRefreshResult(len(streams))
    if s.cfg.OnRefreshDone != nil {
        s.cfg.OnRefreshDone(s.cfg.ID, "", len(streams))
    }
    return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
    streams, err := s.cfg.StreamStore.ListBySource(ctx, string(PluginType), s.cfg.ID)
    if err != nil {
        return nil, err
    }
    ids := make([]string, len(streams))
    for i, st := range streams {
        ids[i] = st.ID
    }
    return ids, nil
}

func (s *Source) DeleteStreams(ctx context.Context) error {
    return s.cfg.StreamStore.DeleteBySource(ctx, string(PluginType), s.cfg.ID)
}

func deterministicID(sourceID, key string) string {
    h := sha256.Sum256([]byte(sourceID + ":" + key))
    return fmt.Sprintf("%x", h[:16])
}

// Self-register with init().
func init() {
    source.DefaultRegistry.RegisterPlugin(source.PluginRegistration{
        Descriptor: source.PluginDescriptor{
            Type:        PluginType,
            Label:       "My Plugin",
            ShortLabel:  "MINE",
            Color:       "#2196f3",
            Version:     "1.0.0",
            Description: "A minimal example source plugin",
        },
        // Factory omitted here — for self-contained plugins, the factory
        // is typically registered in the main wiring (sources.go) because
        // it needs injected dependencies like StreamStore.
    })
}
```

## Complete Example with Config Fields

A source that takes a URL and credentials:

```go
package iptvsource

import (
    "context"
    "fmt"
    "net/http"

    "github.com/mcnairstudios/mediahub/pkg/media"
    "github.com/mcnairstudios/mediahub/pkg/source"
    "github.com/mcnairstudios/mediahub/pkg/store"
)

const PluginType source.SourceType = "iptv"

type Config struct {
    ID            string
    Name          string
    IsEnabled     bool
    URL           string
    Username      string
    Password      string
    StreamStore   store.StreamStore
    OnRefreshDone func(sourceID, etag string, streamCount int)
}

type Source struct {
    source.BaseSource
    cfg    Config
    client *http.Client
}

func New(cfg Config) *Source {
    return &Source{
        BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, PluginType, cfg.IsEnabled, 0),
        cfg:        cfg,
        client:     http.DefaultClient,
    }
}

func (s *Source) Refresh(ctx context.Context) error {
    // Fetch streams from provider using s.cfg.URL, s.cfg.Username, s.cfg.Password
    // Parse response into media.Stream slice
    // ...

    var streams []media.Stream
    // streams = fetchFromProvider(ctx, s.cfg, s.client)

    keepIDs := make([]string, len(streams))
    for i, st := range streams {
        keepIDs[i] = st.ID
    }

    if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
        s.SetError(err.Error())
        return fmt.Errorf("upserting streams: %w", err)
    }

    if _, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, string(PluginType), s.cfg.ID, keepIDs); err != nil {
        s.SetError(err.Error())
        return fmt.Errorf("deleting stale streams: %w", err)
    }

    s.SetRefreshResult(len(streams))
    if s.cfg.OnRefreshDone != nil {
        s.cfg.OnRefreshDone(s.cfg.ID, "", len(streams))
    }
    return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
    streams, err := s.cfg.StreamStore.ListBySource(ctx, string(PluginType), s.cfg.ID)
    if err != nil {
        return nil, err
    }
    ids := make([]string, len(streams))
    for i, st := range streams {
        ids[i] = st.ID
    }
    return ids, nil
}

func (s *Source) DeleteStreams(ctx context.Context) error {
    return s.cfg.StreamStore.DeleteBySource(ctx, string(PluginType), s.cfg.ID)
}

func init() {
    source.DefaultRegistry.RegisterPlugin(source.PluginRegistration{
        Descriptor: source.PluginDescriptor{
            Type:        PluginType,
            Label:       "IPTV Provider",
            ShortLabel:  "IPTV",
            Color:       "#ff5722",
            Version:     "1.0.0",
            Description: "Connect to a generic IPTV provider",
            ConfigFields: []source.ConfigField{
                {
                    Key:         "url",
                    Label:       "Server URL",
                    Type:        source.FieldURL,
                    Required:    true,
                    Placeholder: "https://provider.example.com/api",
                },
                {
                    Key:   "username",
                    Label: "Username",
                    Type:  source.FieldText,
                },
                {
                    Key:   "password",
                    Label: "Password",
                    Type:  source.FieldPassword,
                },
                {
                    Key:     "max_streams",
                    Label:   "Max Concurrent Streams",
                    Type:    source.FieldNumber,
                    Default: "0",
                    HelpText: "0 = unlimited",
                },
                {
                    Key:     "refresh_interval",
                    Label:   "Refresh Interval",
                    Type:    source.FieldSelect,
                    Default: "24h",
                    Options: []source.Option{
                        {Value: "1h", Label: "Every hour"},
                        {Value: "6h", Label: "Every 6 hours"},
                        {Value: "24h", Label: "Every 24 hours"},
                    },
                },
            },
        },
    })
}
```

## Complete Example with Custom UI Component

A source with a custom device picker:

```go
package devicesource

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/mcnairstudios/mediahub/pkg/media"
    "github.com/mcnairstudios/mediahub/pkg/source"
    "github.com/mcnairstudios/mediahub/pkg/store"
)

const PluginType source.SourceType = "devicepicker"

type Config struct {
    ID          string
    Name        string
    IsEnabled   bool
    Devices     []string // selected device IDs from custom UI
    StreamStore store.StreamStore
    OnRefreshDone func(sourceID, etag string, streamCount int)
}

type Source struct {
    source.BaseSource
    cfg Config
}

func New(cfg Config) *Source {
    return &Source{
        BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, PluginType, cfg.IsEnabled, 0),
        cfg:        cfg,
    }
}

func (s *Source) Refresh(ctx context.Context) error {
    // Fetch streams from selected devices
    // ...
    s.SetRefreshResult(0)
    return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
    streams, err := s.cfg.StreamStore.ListBySource(ctx, string(PluginType), s.cfg.ID)
    if err != nil {
        return nil, err
    }
    ids := make([]string, len(streams))
    for i, st := range streams {
        ids[i] = st.ID
    }
    return ids, nil
}

func (s *Source) DeleteStreams(ctx context.Context) error {
    return s.cfg.StreamStore.DeleteBySource(ctx, string(PluginType), s.cfg.ID)
}

// discoverHandler is a custom API route that discovers devices on the network.
// Mounted at: GET /api/sources/devicepicker/discover
func discoverHandler(w http.ResponseWriter, r *http.Request) {
    devices := []map[string]string{
        {"id": "dev-001", "name": "Living Room Tuner", "ip": "192.168.1.50"},
        {"id": "dev-002", "name": "Bedroom Tuner", "ip": "192.168.1.51"},
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(devices)
}

// frontendJS is the JavaScript for the custom "device-picker" component.
// It calls the custom /discover route and renders a device selection UI.
var frontendJS = []byte(`
(function() {
    // Register the "device-picker" custom component.
    // The frontend calls this when rendering a FieldCustom with
    // Component: "device-picker".
    //
    // container: the DOM element to render into
    // value:     current config value (JSON string of selected device IDs)
    // onChange:  callback to update the config value
    window.registerSourceComponent("device-picker", function(container, value, onChange) {
        var selected = value ? JSON.parse(value) : [];

        var btn = document.createElement("button");
        btn.textContent = "Discover Devices";
        btn.onclick = function() {
            fetch("/api/sources/devicepicker/discover")
                .then(function(r) { return r.json(); })
                .then(function(devices) {
                    container.innerHTML = "";
                    devices.forEach(function(dev) {
                        var label = document.createElement("label");
                        var cb = document.createElement("input");
                        cb.type = "checkbox";
                        cb.checked = selected.indexOf(dev.id) !== -1;
                        cb.onchange = function() {
                            if (cb.checked) {
                                selected.push(dev.id);
                            } else {
                                selected = selected.filter(function(id) { return id !== dev.id; });
                            }
                            onChange(JSON.stringify(selected));
                        };
                        label.appendChild(cb);
                        label.appendChild(document.createTextNode(" " + dev.name + " (" + dev.ip + ")"));
                        container.appendChild(label);
                        container.appendChild(document.createElement("br"));
                    });
                });
        };
        container.appendChild(btn);
    });
})();
`)

func init() {
    source.DefaultRegistry.RegisterPlugin(source.PluginRegistration{
        Descriptor: source.PluginDescriptor{
            Type:        PluginType,
            Label:       "Network Devices",
            ShortLabel:  "DEVICES",
            Color:       "#009688",
            Version:     "1.0.0",
            Description: "Discover and import streams from network devices",
            ConfigFields: []source.ConfigField{
                {
                    Key:       "devices",
                    Label:     "Devices",
                    Type:      source.FieldCustom,
                    Component: "device-picker",
                    Required:  true,
                    HelpText:  "Click Discover to find devices on your network",
                },
            },
        },
        CustomRoutes: []source.CustomRoute{
            {
                Method:  "GET",
                Pattern: "discover",
                Handler: http.HandlerFunc(discoverHandler),
            },
        },
        FrontendJS: frontendJS,
    })
}
```

### Key points for custom components

- `CustomRoute.Pattern` is a relative path. `"discover"` becomes `/api/sources/{type}/discover`.
- `CustomRoute.Handler` is typed as `any` to avoid importing `net/http` in the source package. The API layer type-asserts it to `http.HandlerFunc`.
- `FrontendJS` bytes are served to the browser and executed at page load.
- The JavaScript must call `window.registerSourceComponent(componentName, renderFn)` where `componentName` matches the `Component` field value.
