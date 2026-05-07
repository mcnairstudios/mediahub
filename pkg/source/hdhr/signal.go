package hdhr

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SignalInfo holds parsed tuner status from an HDHomeRun device.
type SignalInfo struct {
	Available  bool    `json:"available"`
	Lock       bool    `json:"lock"`
	LevelPct   int     `json:"level_pct"`
	QualityPct int     `json:"quality_pct"`
	SymbolPct  int     `json:"symbol_pct"`
	FreqMHz    float64 `json:"freq_mhz"`
	Msys       string  `json:"msys"`
	Tuner      int     `json:"tuner"`
}

var tdRegexp = regexp.MustCompile(`<td>([^<]*)</td>`)

// parseTunerHTML parses the HTML table returned by /tuners.html?page=tunerN.
// It extracts key-value pairs from <tr><td>Key</td><td>Value</td></tr> rows.
func parseTunerHTML(html string) map[string]string {
	result := make(map[string]string)
	rows := strings.Split(html, "</tr>")
	for _, row := range rows {
		matches := tdRegexp.FindAllStringSubmatch(row, -1)
		if len(matches) >= 2 {
			key := strings.TrimSpace(matches[0][1])
			val := strings.TrimSpace(matches[1][1])
			result[key] = val
		}
	}
	return result
}

// parsePercentage extracts the leading percentage from strings like "75% (-8.3 dBmV)".
func parsePercentage(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" {
		return 0
	}
	idx := strings.Index(s, "%")
	if idx < 0 {
		return 0
	}
	v, err := strconv.Atoi(s[:idx])
	if err != nil {
		return 0
	}
	return v
}

// parseFrequencyHz parses a frequency string like "506000000 Hz" and returns MHz.
func parseFrequencyHz(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, " Hz")
	s = strings.TrimSuffix(s, "Hz")
	s = strings.TrimSpace(s)
	hz, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return hz / 1e6
}

// modulationToMsys maps HDHR modulation lock values to DVB system names.
func modulationToMsys(mod string) string {
	mod = strings.ToLower(strings.TrimSpace(mod))
	if mod == "" || mod == "none" {
		return ""
	}
	switch {
	case strings.HasPrefix(mod, "t2"):
		return "DVB-T2"
	case strings.HasPrefix(mod, "t-") || strings.HasPrefix(mod, "t8") || mod == "8vsb":
		return "DVB-T"
	case strings.HasPrefix(mod, "s2"):
		return "DVB-S2"
	case strings.HasPrefix(mod, "s-"):
		return "DVB-S"
	case strings.HasPrefix(mod, "a") || strings.Contains(mod, "qam"):
		return "DVB-C"
	default:
		return strings.ToUpper(mod)
	}
}

// QueryTunerSignal fetches signal info for a specific tuner on an HDHR device.
// baseURL should be like "http://192.168.1.104". tuner is 0-based.
func QueryTunerSignal(client *http.Client, baseURL string, tuner int, timeout time.Duration) (*SignalInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	url := fmt.Sprintf("%s/tuners.html?page=tuner%d", baseURL, tuner)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching tuner %d status: %w", tuner, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tuner %d returned HTTP %d", tuner, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tuner %d response: %w", tuner, err)
	}

	fields := parseTunerHTML(string(body))

	freq := fields["Frequency"]
	if freq == "" || freq == "none" {
		return &SignalInfo{Available: false, Tuner: tuner}, nil
	}

	mod := fields["Modulation Lock"]
	if mod == "" || mod == "none" {
		return &SignalInfo{Available: false, Tuner: tuner}, nil
	}

	return &SignalInfo{
		Available:  true,
		Lock:       mod != "" && mod != "none",
		LevelPct:   parsePercentage(fields["Signal Strength"]),
		QualityPct: parsePercentage(fields["Signal Quality"]),
		SymbolPct:  parsePercentage(fields["Symbol Quality"]),
		FreqMHz:    parseFrequencyHz(freq),
		Msys:       modulationToMsys(mod),
		Tuner:      tuner,
	}, nil
}

// QueryDeviceSignal tries all tuners (0 to tunerCount-1) and returns the first
// one that has an active signal lock. If tunerCount is 0, it tries up to 6 tuners.
func QueryDeviceSignal(client *http.Client, baseURL string, tunerCount int, timeout time.Duration) (*SignalInfo, error) {
	if tunerCount <= 0 {
		tunerCount = 6
	}
	for i := 0; i < tunerCount; i++ {
		info, err := QueryTunerSignal(client, baseURL, i, timeout)
		if err != nil {
			continue
		}
		if info.Available {
			return info, nil
		}
	}
	return &SignalInfo{Available: false}, nil
}
