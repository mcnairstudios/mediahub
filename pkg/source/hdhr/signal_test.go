package hdhr

import "testing"

func TestParseTunerHTML(t *testing.T) {
	html := `<html><body><table>
<tr><td>Frequency</td><td>506000000 Hz</td></tr>
<tr><td>Modulation Lock</td><td>t2-8qam</td></tr>
<tr><td>Signal Strength</td><td>75% (-8.3 dBmV)</td></tr>
<tr><td>Signal Quality</td><td>100% (35.9 dB)</td></tr>
<tr><td>Symbol Quality</td><td>100%</td></tr>
<tr><td>Resource Lock</td><td>192.168.1.100</td></tr>
</table></body></html>`

	fields := parseTunerHTML(html)
	if fields["Frequency"] != "506000000 Hz" {
		t.Errorf("Frequency = %q, want %q", fields["Frequency"], "506000000 Hz")
	}
	if fields["Modulation Lock"] != "t2-8qam" {
		t.Errorf("Modulation Lock = %q, want %q", fields["Modulation Lock"], "t2-8qam")
	}
	if fields["Signal Strength"] != "75% (-8.3 dBmV)" {
		t.Errorf("Signal Strength = %q, want %q", fields["Signal Strength"], "75% (-8.3 dBmV)")
	}
	if fields["Signal Quality"] != "100% (35.9 dB)" {
		t.Errorf("Signal Quality = %q, want %q", fields["Signal Quality"], "100% (35.9 dB)")
	}
	if fields["Symbol Quality"] != "100%" {
		t.Errorf("Symbol Quality = %q, want %q", fields["Symbol Quality"], "100%")
	}
}

func TestParsePercentage(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"75% (-8.3 dBmV)", 75},
		{"100% (35.9 dB)", 100},
		{"100%", 100},
		{"0%", 0},
		{"none", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parsePercentage(tt.input)
		if got != tt.want {
			t.Errorf("parsePercentage(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseFrequencyHz(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"506000000 Hz", 506.0},
		{"474000000Hz", 474.0},
		{"none", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseFrequencyHz(tt.input)
		if got != tt.want {
			t.Errorf("parseFrequencyHz(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestModulationToMsys(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"t2-8qam", "DVB-T2"},
		{"t-8vsb", "DVB-T"},
		{"8vsb", "DVB-T"},
		{"s2-qpsk", "DVB-S2"},
		{"a-256qam", "DVB-C"},
		{"none", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := modulationToMsys(tt.input)
		if got != tt.want {
			t.Errorf("modulationToMsys(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
