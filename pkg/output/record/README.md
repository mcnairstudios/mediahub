# pkg/output/record — Recording Output Plugin

## Purpose
Records media to disk as mp4/aac. This is the "always recording" plugin that runs alongside every playback session. The record button just preserves the file on cleanup.

## Responsibilities
- Receive encoded video/audio packets via OutputPlugin interface
- Write to mp4 file with AAC audio (recording format from settings)
- Track recording state (active, preserved)
- Support being added mid-stream (short-press record)
- Graceful stop (finalize mp4 container)

## Does NOT
- Serve content via HTTP — recordings are played back as input sources
- Decode or encode — the DecodeBridge handles that
- Manage recording metadata — the recording store handles that

## Key Design (from RECORDING-DESIGN.md)
- Every playback ALWAYS records to a temp file
- Record button = "preserve this file, don't delete on cleanup"
- Short press = record from now (plugin added mid-stream to FanOut)
- Long press = preserve from beginning (rename existing temp file)
- Auto-stop: EPG end time or 4-hour fallback
- Recording output: `<recorddir>/stream/<streamID>/active/source.mp4`
- Completed recording: moved to `<recorddir>/recordings/`

## Reference Implementation
New — no direct equivalent in tvproxy (tvproxy's recording was broken). Uses StreamMuxer("mp4") from pkg/av/mux.
