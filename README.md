# MediaHub

Media hub connecting stream sources to playback sinks with intelligent format negotiation. Sources feed into a unified media cloud, outputs deliver to any client.

## Architecture

```mermaid
graph TB
    subgraph "Source Plugins (Input)"
        SATIP[SAT/IP]
        HDHR[HDHomeRun]
        M3U[IPTV M3U]
        XTREAM[IPTV Xtream]
        TVPS[TVProxy Streams]
        FUTURE_IN[Future: Cameras, HDMI, Pluto, etc.]
    end

    subgraph "Source Stream Profiles"
        SSP[Normalize to consistent internal format]
    end

    SATIP --> SSP
    HDHR --> SSP
    M3U --> SSP
    XTREAM --> SSP
    TVPS --> SSP
    FUTURE_IN --> SSP

    subgraph "Media Cloud (Core)"
        STORE[(Media Store<br/>bolt key-value)]
        CACHE[Caching Layers<br/>EPG / TMDB / Probe]
        STORE --- CACHE
    end

    SSP --> STORE

    subgraph "Session Manager"
        SM[Session per Stream]
        DEMUX[Demuxer]
        BRIDGE[DecodeBridge<br/>decode → process → encode]
        FANOUT[FanOut<br/>one decode → N outputs]
        SM --> DEMUX --> BRIDGE --> FANOUT
    end

    STORE --> SM

    subgraph "Output Plugins (Delivery)"
        MSE[MSE Plugin<br/>fMP4 segments]
        HLS[HLS Plugin<br/>MPEG-TS + m3u8]
        STREAM[Stream Plugin<br/>chunked HTTP]
        RECORD[Recording Plugin<br/>mp4/aac to disk]
        FUTURE_OUT[Future: WebRTC, DASH]
    end

    FANOUT --> MSE
    FANOUT --> HLS
    FANOUT --> STREAM
    FANOUT --> RECORD
    FANOUT --> FUTURE_OUT

    subgraph "Client Detection"
        CD[Match rules + port]
        CP[Client Profile<br/>codec / container / delivery]
        CD --> CP
    end

    subgraph "Strategy"
        STRAT[Copy vs Transcode<br/>source + client → decision]
    end

    CP --> STRAT
    STRAT --> SM

    subgraph "Output Clients"
        BROWSER[Browser UI<br/>MSE / HLS]
        JELLYFIN[Jellyfin Emulation<br/>HLS]
        HDHR_OUT[HDHR Emulation<br/>Plex / Channels DVR]
        DLNA[DLNA<br/>TVs / Quest]
        M3U_OUT[M3U + XMLTV Output]
    end

    MSE --> BROWSER
    HLS --> JELLYFIN
    HLS --> BROWSER
    STREAM --> HDHR_OUT
    STREAM --> DLNA
    STREAM --> M3U_OUT
```

## Package Map

```
pkg/
  source/     Source plugin interfaces + registry
  output/     Output plugin interfaces + FanOut + registry
  media/      Shared media types (codecs, streams, probe)
  store/      Persistence interfaces + in-memory impl
  session/    Session manager (one session per stream, FanOut)
  config/     Environment-based configuration
  strategy/   Copy vs transcode decision engine
  client/     Client detection + profile resolution
```

## Data Flow

```mermaid
sequenceDiagram
    participant User
    participant Client as Client Detector
    participant Strategy
    participant Session as Session Manager
    participant Demux as Demuxer
    participant Bridge as DecodeBridge
    participant FanOut
    participant MSE as MSE Plugin
    participant Record as Recording Plugin

    User->>Client: HTTP request (User-Agent, port)
    Client->>Strategy: client profile + probe data
    Strategy->>Session: decision (copy/transcode)
    Session->>Demux: open stream URL
    Demux->>Bridge: compressed packets
    Bridge->>FanOut: encoded packets
    FanOut->>MSE: packets (browser delivery)
    FanOut->>Record: packets (always recording)
    MSE->>User: fMP4 segments via HTTP
    
    Note over User,Record: User presses Record
    User->>Session: preserve recording
    Note over Record: File kept on cleanup
```

## Recording Flow

```mermaid
stateDiagram-v2
    [*] --> Playing: User clicks play
    Playing --> Playing: Always recording to temp file
    Playing --> RecordingActive: Short press record (from now)
    Playing --> RecordingActive: Long press record (entire session)
    Playing --> Cleanup: User closes player
    Cleanup --> [*]: Temp file deleted

    RecordingActive --> RecordingBackground: User closes player
    RecordingActive --> Preserved: User stops recording
    RecordingBackground --> Preserved: EPG end / 4hr timeout
    Preserved --> [*]: File moved to recordings/
```

## Key Principles

1. **Modularity protects working code.** Each component has clean boundaries. Changing one output plugin cannot break another.
2. **The media cloud is the heart.** Everything else is a plugin — inputs feed it, outputs consume it.
3. **One decode, many outputs.** Decoded frames are the shared resource. FanOut distributes to recording + delivery.
4. **Recording is always happening.** The record button just preserves what's already being written.
5. **Recordings are input sources.** Playing back a recording goes through the normal pipeline.
6. **Sessions keyed by stream.** Multiple users watching the same stream share one session (one decode).

## Build & Test

```bash
go test ./pkg/... -v
```
