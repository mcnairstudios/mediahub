# pkg/source/satip — SAT>IP Source Plugin

## Purpose
Provides streams from SAT>IP tuner devices (DVB-T/T2/S/S2/C). SAT>IP servers expose DVB services over RTSP/RTP on the local network.

## Responsibilities
- Track SAT>IP device configuration (host, HTTP port, transmitter file)
- List previously scanned streams from the stream store
- Delete/clear streams belonging to this source
- Provide deterministic stream IDs based on source ID + DVB service ID
- Classify streams into groups (SD, HD, Radio) by DVB service type
- Network discovery of SAT>IP devices (stub, full SSDP discovery later)

## Current State (MVP)
Refresh is a no-op stub. Full DVB-SI scanning (NIT/SDT/PMT via tvsatipscan) will be integrated later. Streams can be pre-populated in the store from a previous scan.

## Stream URL Format
SAT>IP streams use RTSP URLs with DVB tuning parameters:
```
rtsp://{host}/?freq={freq}&msys={msys}&mtype={mtype}&pids={pids}&bw={bw}
```

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)
- `source.Clearable`
- `source.Discoverable`

## Does NOT
- Own the stream store — uses the provided StreamStore interface
- Perform DVB scanning yet — Refresh is a stub for MVP
- Handle RTSP session management — that's the session layer's job

## Reference
Port from tvproxy's pkg/service/satip.go and pkg/tvsatipscan/.
