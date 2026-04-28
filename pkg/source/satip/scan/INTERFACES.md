# scan -- Public API

## Functions

### Scan

```go
func Scan(host string, httpPort int, cfg Config) (*ScanResult, error)
```

Full DVB scan. Resolves muxes (from transmitter file or NIT discovery), scans each mux for channels via RTSP, returns all discovered channels with their stream components.

### DiscoverMuxes

```go
func DiscoverMuxes(host string, httpPort int, cfg Config) ([]Transponder, string, error)
```

Resolve muxes without scanning for channels. Returns transponder list, network name, and error. Uses transmitter file if `cfg.TransmitterFile` is set, otherwise discovers via NIT.

### ListTransmitters

```go
func ListTransmitters(system string) ([]string, error)
```

List available transmitter file names for a delivery system directory (e.g. "dvb-t", "dvb-s"). Reads from `DVBTablesDir/<system>/`.

### ParseTransmitterFile

```go
func ParseTransmitterFile(transmitterFile string) ([]Transponder, error)
```

Parse a dvb-scan transmitter file into transponder definitions. Path is relative to `DVBTablesDir`. Supports DVB-T, DVB-T2, DVB-S, DVB-S2, DVB-C formats.

### Satellites

```go
func Satellites() []string
```

List known satellite identifiers (e.g. "S28.2E", "S19.2E", "S13E").

### QuerySignal

```go
func QuerySignal(rtspURL string, timeout time.Duration) (*SignalInfo, error)
```

Query tuner signal strength, quality, and lock status via RTSP DESCRIBE.

### ServiceTypeName / StreamTypeName

```go
func ServiceTypeName(t uint8) string
func StreamTypeName(t uint8) string
```

Human-readable names for DVB service type codes (0x01="TV", 0x11="HD-TV", etc.) and stream type codes (0x1b="H.264 Video", 0x0f="AAC Audio", etc.).

## Variables

### DVBTablesDir

```go
var DVBTablesDir string
```

Resolved at init from: `MEDIAHUB_DVB_TABLES_DIR` env var, then `~/dvb`, then `/usr/share/dvb`.

## Types

### Config

```go
type Config struct {
    SeedTimeout     time.Duration
    MuxTimeout      time.Duration
    Timeout         time.Duration
    Parallel        int
    Verbose         bool
    Satellite       string
    TransmitterFile string
    Log             zerolog.Logger
    OnMuxScanned    func(done, total int)
}
```

- `SeedTimeout` -- timeout for seed frequency probes (pass 1)
- `MuxTimeout` -- timeout for NIT-discovered mux probes (pass 2)
- `Timeout` -- timeout for full channel scan per mux
- `Parallel` -- max concurrent mux scans (default 4)
- `Satellite` -- manual satellite selection (e.g. "S28.2E"), skips auto-detection
- `TransmitterFile` -- path relative to DVBTablesDir; bypasses NIT discovery
- `OnMuxScanned` -- progress callback during channel scan phase

### ScanResult

```go
type ScanResult struct {
    Host          string
    NetworkName   string
    Muxes         []Transponder
    Channels      []Channel
    NoSignalMuxes []Transponder
    ErrorMuxes    []Transponder
}
```

### Transponder

```go
type Transponder struct {
    FreqMHz      float64
    System       string        // dvbt, dvbt2, dvbs, dvbs2, dvbc, dvbc2
    Modulation   string        // qpsk, 64qam, 256qam, etc.
    BandwidthMHz int           // DVB-T/T2 only
    SymbolRateKS int           // DVB-S/C only
    Polarization string        // DVB-S only: h, v
    PLPID        int           // DVB-T2 only
}
```

Methods: `String()`, `RTSPURL(host, pids)`, `MuxKey()`.

### Channel

```go
type Channel struct {
    Name        string
    ServiceID   uint16
    ServiceType uint8
    Encrypted   bool
    PMTPID      uint16
    PCRPID      uint16
    Streams     []StreamComponent
    Transponder Transponder
}
```

Method: `RTSPURL(host)` -- builds RTSP URL with all stream PIDs.

### StreamComponent

```go
type StreamComponent struct {
    PID        uint16
    StreamType uint8
    Language   string
    AudioType  uint8
    Label      string
    Category   string    // video, audio, subtitle, teletext
    TypeName   string    // human-readable stream type
}
```

### SignalInfo

```go
type SignalInfo struct {
    Lock       bool
    Level      int
    Quality    int
    BER        int
    FeID       int
    FreqMHz    float64
    BwMHz      int
    Msys       string
    Mtype      string
    PLPID      string
    T2ID       string
    BitratKbps int
    Active     bool
    Server     string
}
```

Methods: `LevelPct()`, `QualityPct()`.
