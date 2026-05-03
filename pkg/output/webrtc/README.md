# pkg/output/webrtc — WebRTC Output Plugin

## Purpose
Delivers media to browsers via WebRTC using the WHEP (WebRTC-HTTP Egress Protocol) signalling protocol. Receives encoded video (H.264) and audio (Opus) packets via the OutputPlugin interface, packetizes them as RTP, and sends them to browsers via a pion/webrtc PeerConnection. Ultra-low-latency delivery compared to MSE or HLS.

## Signalling (WHEP)

The plugin implements WHEP via ServeHTTP:

- **POST** with an SDP offer body creates a PeerConnection and returns the SDP answer (201 Created)
- **DELETE** tears down the active connection (204 No Content)

The router mounts the plugin's HTTP handler at the appropriate prefix.

## Media Flow

1. Pipeline calls PushVideo/PushAudio with encoded packet data
2. H.264/HEVC NAL units are extracted (supports both Annex B and AVCC framing)
3. NALUs are packetized as RTP with FU-A fragmentation for large NALUs (RFC 6184 for H.264, RFC 7798 for HEVC)
4. Audio is packetized as RTP with Opus payload type
5. RTP packets are written to pion TrackLocalStaticRTP tracks
6. pion handles DTLS/SRTP encryption and ICE transport to the browser

## Codec Support

- Video: H.264 and HEVC/H.265 (payload type 96, clock rate 90000). Codec selected from PluginConfig.Video.Codec.
- Audio: Opus (payload type 97, clock rate 48000, stereo)

## RTP Timestamp Handling

- Video and audio timestamps derived from source PTS (presentation timestamps), not hardcoded frame rate increments
- PTS base is captured from the first packet after connection or seek, ensuring timestamps start at 0
- ResetForSeek clears PTS base, sequence numbers, and timestamps so post-seek packets start fresh
- ptsToRTP converts from 90kHz PTS timebase to the track's clock rate

## ICE Configuration

Uses Google's public STUN server (`stun:stun.l.google.com:19302`) for NAT traversal. TURN servers are not configured by default.

## Does NOT
- Know about MSE, HLS, DASH, stream copy, or recording — it's one delivery plugin
- Manage sessions — the session manager handles lifecycle
- Decode or encode — receives already-encoded packets

## Key Integration Points
- **Input**: Receives packets from FanOut via PushVideo/PushAudio
- **Output**: Serves WHEP signalling via ServeHTTP (implements ServablePlugin)
- **Transport**: Uses pion/webrtc for PeerConnection, DTLS/SRTP, ICE
