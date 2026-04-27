# pkg/av/selector interfaces

## Dependencies

- `github.com/mcnairstudios/mediahub/pkg/media` -- `media.AudioTrack` type

## Exported API

```go
type AudioPrefs struct {
    Language string
}

func SelectAudio(tracks []media.AudioTrack, prefs AudioPrefs) int
```

## Used By

- Pipeline setup code to choose which audio stream index to demux
