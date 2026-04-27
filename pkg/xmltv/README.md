# xmltv

XMLTV EPG (Electronic Program Guide) parser. Reads standard XMLTV XML format and returns structured channel and programme data.

## Usage

```go
channels, programmes, err := xmltv.Parse(reader)
```

`Parse` accepts an `io.Reader` containing XMLTV XML and returns:
- `[]Channel` — channel ID, display name, icon URL
- `[]Programme` — title, subtitle, description, start/stop times, categories, rating, episode number, credits, new/rerun status

## XMLTV datetime format

`20260425180000 +0100` — 14-digit datetime followed by a space and a 5-character timezone offset (`+HHMM` or `-HHMM`).

## IsNew logic

A programme is considered new (`IsNew = true`) when it has an `<episode-num>` element and no `<previously-shown>` element. Programmes without episode numbers default to `IsNew = false`.

## Dependencies

Standard library only: `encoding/xml`, `time`, `io`, `fmt`, `strconv`, `strings`.
