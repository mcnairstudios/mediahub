# validate - Interfaces

## Public Types

```go
type ValidationError struct {
    Field   string
    Message string
}
```

## Public Functions

```go
func ValidateHLSPlaylist(data []byte) []ValidationError
func ValidateMPD(data []byte) []ValidationError
func ValidateFMP4Init(data []byte) []ValidationError
func ValidateFMP4Segment(data []byte) []ValidationError
```

## Dependencies

None. Pure standard library (bytes, encoding/binary, encoding/xml, regexp, strconv, strings, fmt).

## Consumed By

- Test suites (regression tests from playback bug reports)
- Future: debug HTTP endpoint for live manifest inspection
