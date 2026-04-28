# scan

DVB scanning for SAT>IP devices. Tunes to terrestrial (DVB-T/T2), satellite (DVB-S/S2), and cable (DVB-C) frequencies via RTSP, then probes for NIT/SDT/PMT tables to discover multiplexes and channels.

## How it works

1. **Resolve muxes** -- either from a transmitter file (`ParseTransmitterFile`) or by auto-discovery via NIT (Network Information Table) from the SAT>IP device.
2. **Scan muxes in parallel** -- connects to each frequency over RTSP/TCP, receives MPEG-TS packets, and demuxes PAT/NIT/SDT/PMT tables using go-astits.
3. **Build channel list** -- maps service IDs from PAT to names from SDT, then resolves elementary streams (video, audio, subtitle, teletext) from PMT.
4. **Satellite detection** -- for DVB-S/S2, probes known transponders on European satellites (Astra 28.2E, 19.2E, Hotbird 13E, Thor 0.8W, Hispasat 30W) to identify the connected dish.

Discovery uses a two-pass approach: pass 1 scans seed frequencies with short timeouts to find active muxes and retrieve the NIT, then pass 2 retries promising candidates (NIT-mentioned, frequency-adjacent) with longer timeouts.

## Key functions

- `Scan(host, httpPort, cfg)` -- full scan: resolve muxes, scan all, return channels
- `DiscoverMuxes(host, httpPort, cfg)` -- resolve muxes only (from file or NIT), no channel scan
- `ListTransmitters(system)` -- list available transmitter files for a delivery system (e.g. "dvb-t")
- `ParseTransmitterFile(file)` -- parse a dvb-scan transmitter file into transponder definitions
- `Satellites()` -- list known satellite identifiers (S28.2E, S19.2E, etc.)
- `QuerySignal(rtspURL, timeout)` -- query tuner signal strength/quality via RTSP DESCRIBE
- `ServiceTypeName(t)` / `StreamTypeName(t)` -- human-readable names for DVB service/stream type codes

## DVB tables directory

Transmitter files follow the standard dvb-scan format (INI-style blocks with DELIVERY_SYSTEM, FREQUENCY, BANDWIDTH_HZ, etc.).

The directory is resolved in order:

1. `MEDIAHUB_DVB_TABLES_DIR` environment variable
2. `~/dvb` if it exists
3. `/usr/share/dvb` (Linux default from dvb-apps/dtv-scan-tables)

Subdirectories: `dvb-t/`, `dvb-s/`, `dvb-c/`, `dvb-t2/`, etc.
