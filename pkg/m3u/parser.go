package m3u

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

type Entry struct {
	Name       string
	URL        string
	Group      string
	TvgID      string
	TvgName    string
	TvgLogo    string
	Duration   int
	Attributes map[string]string
}

func Parse(r io.Reader) ([]Entry, error) {
	var entries []Entry
	var pending *Entry

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#EXTM3U") {
			continue
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			e := parseExtinf(line)
			if e != nil {
				pending = e
			}
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		if pending != nil {
			pending.URL = line
			entries = append(entries, *pending)
			pending = nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func parseExtinf(line string) *Entry {
	rest := line[len("#EXTINF:"):]

	commaIdx := findDisplayNameComma(rest)
	if commaIdx < 0 {
		return nil
	}

	durationAndAttrs := rest[:commaIdx]
	displayName := strings.TrimSpace(rest[commaIdx+1:])

	duration, attrs := parseDurationAndAttrs(durationAndAttrs)
	if duration == nil {
		return nil
	}

	entry := &Entry{
		Name:       displayName,
		Duration:   *duration,
		Attributes: make(map[string]string),
	}

	for k, v := range attrs {
		switch k {
		case "tvg-id":
			entry.TvgID = v
		case "tvg-name":
			entry.TvgName = v
		case "tvg-logo":
			entry.TvgLogo = v
		case "group-title":
			entry.Group = v
		default:
			entry.Attributes[k] = v
		}
	}

	return entry
}

func findDisplayNameComma(s string) int {
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				return i
			}
		}
	}
	return -1
}

func parseDurationAndAttrs(s string) (*int, map[string]string) {
	attrs := make(map[string]string)
	s = strings.TrimSpace(s)

	spaceIdx := strings.IndexByte(s, ' ')
	var durStr string
	var attrStr string

	if spaceIdx < 0 {
		durStr = s
	} else {
		durStr = s[:spaceIdx]
		attrStr = strings.TrimSpace(s[spaceIdx+1:])
	}

	dur, err := strconv.Atoi(durStr)
	if err != nil {
		return nil, nil
	}

	if attrStr != "" {
		parseAttributes(attrStr, attrs)
	}

	return &dur, attrs
}

func parseAttributes(s string, attrs map[string]string) {
	for len(s) > 0 {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}

		eqIdx := strings.IndexByte(s, '=')
		if eqIdx < 0 {
			break
		}

		key := strings.TrimSpace(s[:eqIdx])
		s = s[eqIdx+1:]

		if len(s) == 0 {
			break
		}

		if s[0] == '"' {
			s = s[1:]
			closeQuote := strings.IndexByte(s, '"')
			if closeQuote < 0 {
				attrs[key] = s
				break
			}
			attrs[key] = s[:closeQuote]
			s = s[closeQuote+1:]
		} else {
			spaceIdx := strings.IndexByte(s, ' ')
			if spaceIdx < 0 {
				attrs[key] = s
				break
			}
			attrs[key] = s[:spaceIdx]
			s = s[spaceIdx+1:]
		}
	}
}
