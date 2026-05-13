package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDVBTablesDirEnvVar(t *testing.T) {
	orig := DVBTablesDir
	defer func() { DVBTablesDir = orig }()

	t.Setenv("MEDIAHUB_DVB_TABLES_DIR", "/tmp/test-dvb-tables")
	DVBTablesDir = resolveDVBTablesDir()
	if DVBTablesDir != "/tmp/test-dvb-tables" {
		t.Fatalf("expected /tmp/test-dvb-tables, got %s", DVBTablesDir)
	}
}

func TestDVBTablesDirHomeFallback(t *testing.T) {
	orig := DVBTablesDir
	defer func() { DVBTablesDir = orig }()

	t.Setenv("MEDIAHUB_DVB_TABLES_DIR", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	homeDir := filepath.Join(home, "dvb")
	if info, err := os.Stat(homeDir); err == nil && info.IsDir() {
		DVBTablesDir = resolveDVBTablesDir()
		if DVBTablesDir != homeDir {
			t.Fatalf("expected %s, got %s", homeDir, DVBTablesDir)
		}
	} else {
		DVBTablesDir = resolveDVBTablesDir()
		if DVBTablesDir != "/usr/share/dvb" {
			t.Fatalf("expected /usr/share/dvb fallback, got %s", DVBTablesDir)
		}
	}
}

func TestListTransmitters(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	dvbDir := filepath.Join(home, "dvb", "dvb-t")
	if _, err := os.Stat(dvbDir); err != nil {
		t.Skipf("~/dvb/dvb-t not found, skipping: %v", err)
	}

	orig := DVBTablesDir
	defer func() { DVBTablesDir = orig }()
	DVBTablesDir = filepath.Join(home, "dvb")

	names, err := ListTransmitters("dvb-t")
	if err != nil {
		t.Fatalf("ListTransmitters failed: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected at least one transmitter file in dvb-t")
	}

	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("results not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

func TestListTransmittersNonExistentDir(t *testing.T) {
	orig := DVBTablesDir
	defer func() { DVBTablesDir = orig }()
	DVBTablesDir = "/nonexistent/path"

	_, err := ListTransmitters("dvb-t")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestParseTransmitterFile(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	dvbDir := filepath.Join(home, "dvb", "dvb-t")
	if _, err := os.Stat(dvbDir); err != nil {
		t.Skipf("~/dvb/dvb-t not found, skipping: %v", err)
	}

	orig := DVBTablesDir
	defer func() { DVBTablesDir = orig }()
	DVBTablesDir = filepath.Join(home, "dvb")

	entries, err := os.ReadDir(dvbDir)
	if err != nil {
		t.Fatalf("reading dvb-t dir: %v", err)
	}
	var testFile string
	for _, e := range entries {
		if !e.IsDir() {
			testFile = e.Name()
			break
		}
	}
	if testFile == "" {
		t.Skip("no transmitter files found in dvb-t")
	}

	tps, err := ParseTransmitterFile(filepath.Join("dvb-t", testFile))
	if err != nil {
		t.Fatalf("ParseTransmitterFile(%q) failed: %v", testFile, err)
	}
	if len(tps) == 0 {
		t.Fatalf("expected at least one transponder from %s", testFile)
	}

	for _, tp := range tps {
		if tp.FreqMHz <= 0 {
			t.Errorf("transponder has non-positive frequency: %g", tp.FreqMHz)
		}
		if tp.System == "" {
			t.Error("transponder has empty system")
		}
		if tp.System != "dvbt" && tp.System != "dvbt2" {
			t.Errorf("dvb-t file produced unexpected system: %s", tp.System)
		}
		if tp.BandwidthMHz <= 0 {
			t.Errorf("transponder at %g MHz has non-positive bandwidth: %d", tp.FreqMHz, tp.BandwidthMHz)
		}
	}
}

func TestParseTransmitterFileNonExistent(t *testing.T) {
	orig := DVBTablesDir
	defer func() { DVBTablesDir = orig }()
	DVBTablesDir = "/nonexistent"

	_, err := ParseTransmitterFile("dvb-t/fake-country")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestNormaliseSystem(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"DVBT", "dvbt"},
		{"DVBT2", "dvbt2"},
		{"DVBS", "dvbs"},
		{"DVBS2", "dvbs2"},
		{"DVBC_ANNEX_A", "dvbc"},
		{"DVBC_ANNEX_B", "dvbc"},
		{"DVBC", "dvbc"},
		{"dvbt", "dvbt"},
		{"ATSC", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := normaliseSystem(tt.input)
		if got != tt.want {
			t.Errorf("normaliseSystem(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormaliseModulation(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"QAM/256", "256qam"},
		{"QAM256", "256qam"},
		{"QAM/64", "64qam"},
		{"QAM64", "64qam"},
		{"QAM/32", "32qam"},
		{"QAM/16", "16qam"},
		{"QPSK", "qpsk"},
		{"8VSB", "8vsb"},
		{"UNKNOWN", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := normaliseModulation(tt.input)
		if got != tt.want {
			t.Errorf("normaliseModulation(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalisePolarization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"HORIZONTAL", "h"},
		{"H", "h"},
		{"VERTICAL", "v"},
		{"V", "v"},
		{"CIRCULAR", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalisePolarization(tt.input)
		if got != tt.want {
			t.Errorf("normalisePolarization(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTransponderString(t *testing.T) {
	tests := []struct {
		tp   Transponder
		want string
	}{
		{Transponder{FreqMHz: 474, System: "dvbt", BandwidthMHz: 8}, "474 MHz dvbt bw=8MHz"},
		{Transponder{FreqMHz: 474, System: "dvbt2", BandwidthMHz: 8}, "474 MHz dvbt2 bw=8MHz"},
		{Transponder{FreqMHz: 10714, System: "dvbs2", Polarization: "h", SymbolRateKS: 22000}, "10714 MHz dvbs2 h sr=22000kS/s"},
		{Transponder{FreqMHz: 114, System: "dvbc", SymbolRateKS: 6952}, "114 MHz dvbc sr=6952kS/s"},
	}
	for _, tt := range tests {
		got := tt.tp.String()
		if got != tt.want {
			t.Errorf("Transponder.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestTransponderRTSPURL(t *testing.T) {
	tp := Transponder{FreqMHz: 474, System: "dvbt", Modulation: "64qam", BandwidthMHz: 8}
	url := tp.RTSPURL("192.168.1.50:554", "0,100,101")
	expected := "rtsp://192.168.1.50:554/?freq=474&msys=dvbt&mtype=64qam&pids=0,100,101&bw=8"
	if url != expected {
		t.Errorf("RTSPURL = %q, want %q", url, expected)
	}
}

func TestTransponderRTSPURLSDTPids(t *testing.T) {
	tp := Transponder{FreqMHz: 474, System: "dvbt", Modulation: "64qam", BandwidthMHz: 8}
	url := tp.RTSPURL("192.168.1.50:554", "sdt")
	expected := "rtsp://192.168.1.50:554/?freq=474&msys=dvbt&mtype=64qam&pids=0,16,17&bw=8"
	if url != expected {
		t.Errorf("RTSPURL(sdt) = %q, want %q", url, expected)
	}
}

func TestTransponderRTSPURLSatellite(t *testing.T) {
	tp := Transponder{FreqMHz: 10714, System: "dvbs2", Modulation: "qpsk", SymbolRateKS: 22000, Polarization: "h"}
	url := tp.RTSPURL("192.168.1.50:554", "0,16,17")
	expected := "rtsp://192.168.1.50:554/?freq=10714&msys=dvbs2&mtype=qpsk&pids=0,16,17&pol=h&sr=22000&src=1"
	if url != expected {
		t.Errorf("RTSPURL = %q, want %q", url, expected)
	}
}

func TestTransponderRTSPURLSatelliteExplicitSource(t *testing.T) {
	tp := Transponder{FreqMHz: 10714, System: "dvbs2", Modulation: "qpsk", SymbolRateKS: 22000, Polarization: "h", Source: 3}
	url := tp.RTSPURL("192.168.1.50:554", "0,16,17")
	expected := "rtsp://192.168.1.50:554/?freq=10714&msys=dvbs2&mtype=qpsk&pids=0,16,17&pol=h&sr=22000&src=3"
	if url != expected {
		t.Errorf("RTSPURL = %q, want %q", url, expected)
	}
}

func TestTransponderRTSPURLT2PLP(t *testing.T) {
	tp := Transponder{FreqMHz: 474, System: "dvbt2", Modulation: "256qam", BandwidthMHz: 8, PLPID: 3}
	url := tp.RTSPURL("192.168.1.50:554", "0,16,17")
	expected := "rtsp://192.168.1.50:554/?freq=474&msys=dvbt2&mtype=256qam&pids=0,16,17&bw=8&plp=3"
	if url != expected {
		t.Errorf("RTSPURL = %q, want %q", url, expected)
	}
}

func TestMuxKey(t *testing.T) {
	tests := []struct {
		tp   Transponder
		want string
	}{
		{Transponder{FreqMHz: 474, System: "dvbt"}, "474/dvbt"},
		{Transponder{FreqMHz: 474, System: "dvbt2", PLPID: 0}, "474/dvbt2/0"},
		{Transponder{FreqMHz: 474, System: "dvbt2", PLPID: 3}, "474/dvbt2/3"},
		{Transponder{FreqMHz: 10714, System: "dvbs2"}, "10714/dvbs2"},
		{Transponder{FreqMHz: 114, System: "dvbc"}, "114/dvbc"},
	}
	for _, tt := range tests {
		got := tt.tp.MuxKey()
		if got != tt.want {
			t.Errorf("MuxKey() for %s = %q, want %q", tt.tp.String(), got, tt.want)
		}
	}
}

func TestServiceTypeName(t *testing.T) {
	tests := []struct {
		code uint8
		want string
	}{
		{0x01, "TV"},
		{0x02, "Radio"},
		{0x11, "HD-TV"},
		{0x19, "HD-TV(AVC)"},
		{0x1f, "TV(HEVC)"},
		{0xFF, "0xff"},
	}
	for _, tt := range tests {
		got := ServiceTypeName(tt.code)
		if got != tt.want {
			t.Errorf("ServiceTypeName(0x%02X) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestStreamTypeName(t *testing.T) {
	tests := []struct {
		code uint8
		want string
	}{
		{0x02, "MPEG-2 Video"},
		{0x1b, "H.264 Video"},
		{0x24, "H.265/HEVC Video"},
		{0x0f, "AAC Audio"},
		{0x81, "AC-3 Audio"},
		{0xFE, "0xfe"},
	}
	for _, tt := range tests {
		got := StreamTypeName(tt.code)
		if got != tt.want {
			t.Errorf("StreamTypeName(0x%02X) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestStreamCategory(t *testing.T) {
	tests := []struct {
		code uint8
		want string
	}{
		{0x02, "video"},
		{0x1b, "video"},
		{0x24, "video"},
		{0x03, "audio"},
		{0x0f, "audio"},
		{0x81, "audio"},
		{0x06, ""},
		{0x00, ""},
	}
	for _, tt := range tests {
		got := streamCategory(tt.code)
		if got != tt.want {
			t.Errorf("streamCategory(0x%02X) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestSatellites(t *testing.T) {
	sats := Satellites()
	if len(sats) == 0 {
		t.Fatal("expected at least one satellite")
	}

	for i := 1; i < len(sats); i++ {
		if sats[i] < sats[i-1] {
			t.Fatalf("satellites not sorted: %q before %q", sats[i-1], sats[i])
		}
	}

	found := false
	for _, s := range sats {
		if s == "S28.2E" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected S28.2E in satellite list")
	}
}

func TestDefaultSeeds(t *testing.T) {
	if len(defaultSeeds) == 0 {
		t.Fatal("expected default seeds to be populated")
	}
	if len(defaultSeeds["dvbt"]) == 0 {
		t.Fatal("expected dvbt seeds")
	}
	if len(defaultSeeds["dvbt2"]) == 0 {
		t.Fatal("expected dvbt2 seeds")
	}
	if len(defaultSeeds["dvbc"]) == 0 {
		t.Fatal("expected dvbc seeds")
	}

	for _, tp := range defaultSeeds["dvbt"] {
		if tp.System != "dvbt" {
			t.Errorf("dvbt seed has wrong system: %s", tp.System)
		}
		if tp.FreqMHz < 470 || tp.FreqMHz > 862 {
			t.Errorf("dvbt seed frequency out of UHF range: %g", tp.FreqMHz)
		}
	}
}

func TestChannelRTSPURL(t *testing.T) {
	ch := Channel{
		ServiceID: 1234,
		PMTPID:    100,
		Streams: []StreamComponent{
			{PID: 101},
			{PID: 102},
		},
		Transponder: Transponder{FreqMHz: 474, System: "dvbt", Modulation: "64qam", BandwidthMHz: 8},
	}
	url := ch.RTSPURL("192.168.1.50:554")
	expected := "rtsp://192.168.1.50:554/?freq=474&msys=dvbt&mtype=64qam&pids=0,100,101,102&bw=8"
	if url != expected {
		t.Errorf("Channel.RTSPURL = %q, want %q", url, expected)
	}
}

func TestChannelRTSPURLNoStreams(t *testing.T) {
	ch := Channel{
		ServiceID:   1234,
		PMTPID:      100,
		Transponder: Transponder{FreqMHz: 474, System: "dvbt", Modulation: "64qam", BandwidthMHz: 8},
	}
	url := ch.RTSPURL("192.168.1.50:554")
	expected := "rtsp://192.168.1.50:554/?freq=474&msys=dvbt&mtype=64qam&pids=all&bw=8"
	if url != expected {
		t.Errorf("Channel.RTSPURL (no streams) = %q, want %q", url, expected)
	}
}

func TestWorkerCount(t *testing.T) {
	tests := []struct {
		caps map[string]int
		want int
	}{
		{map[string]int{"dvbt2": 2, "dvbc": 1}, 2},
		{map[string]int{"dvbt": 1}, 1},
		{map[string]int{}, 1},
		{map[string]int{"dvbs2": 4, "dvbt": 2}, 4},
	}
	for _, tt := range tests {
		got := workerCount(tt.caps)
		if got != tt.want {
			t.Errorf("workerCount(%v) = %d, want %d", tt.caps, got, tt.want)
		}
	}
}

func TestSplitHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"192.168.1.50:554", "192.168.1.50"},
		{"192.168.1.50", "192.168.1.50"},
		{"[::1]:554", "[::1]"},
	}
	for _, tt := range tests {
		got := splitHost(tt.input)
		if got != tt.want {
			t.Errorf("splitHost(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"rtsp://192.168.1.50:554/?freq=474", "192.168.1.50:554"},
		{"rtsp://192.168.1.50/?freq=474", "192.168.1.50:554"},
		{"rtsp://myhost:1234/stream=1", "myhost:1234"},
	}
	for _, tt := range tests {
		got := extractHost(tt.input)
		if got != tt.want {
			t.Errorf("extractHost(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBCDToUint32(t *testing.T) {
	tests := []struct {
		input uint32
		want  uint32
	}{
		{0x12345678, 12345678},
		{0x00000000, 0},
		{0x99999999, 99999999},
		{0x10000000, 10000000},
	}
	for _, tt := range tests {
		got := bcdToUint32(tt.input)
		if got != tt.want {
			t.Errorf("bcdToUint32(0x%08X) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestDVBString(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{nil, ""},
		{[]byte{}, ""},
		{[]byte("BBC One"), "BBC One"},
		{[]byte{0x05, 'B', 'B', 'C', ' ', 'O', 'n', 'e'}, "BBC One"},
		{[]byte("  padded  "), "padded"},
	}
	for _, tt := range tests {
		got := dvbString(tt.input)
		if got != tt.want {
			t.Errorf("dvbString(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsModulationType(t *testing.T) {
	valid := []string{"qpsk", "8psk", "16qam", "64qam", "256qam", "8vsb"}
	for _, v := range valid {
		if !isModulationType(v) {
			t.Errorf("expected %q to be a valid modulation type", v)
		}
	}
	if isModulationType("unknown") {
		t.Error("expected 'unknown' to be invalid")
	}
}

func TestSignalInfoPercentages(t *testing.T) {
	s := SignalInfo{Level: 255, Quality: 15}
	if s.LevelPct() != 100 {
		t.Errorf("LevelPct() = %d, want 100", s.LevelPct())
	}
	if s.QualityPct() != 100 {
		t.Errorf("QualityPct() = %d, want 100", s.QualityPct())
	}

	s2 := SignalInfo{Level: 0, Quality: 0}
	if s2.LevelPct() != 0 {
		t.Errorf("LevelPct() = %d, want 0", s2.LevelPct())
	}
	if s2.QualityPct() != 0 {
		t.Errorf("QualityPct() = %d, want 0", s2.QualityPct())
	}
}

func TestParseTransponderEntryDVBT(t *testing.T) {
	fields := map[string]string{
		"DELIVERY_SYSTEM": "DVBT",
		"FREQUENCY":       "474000000",
		"BANDWIDTH_HZ":    "8000000",
		"MODULATION":      "QAM/64",
	}
	tp, ok := parseTransponderEntry(fields)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if tp.System != "dvbt" {
		t.Errorf("system = %s, want dvbt", tp.System)
	}
	if tp.FreqMHz != 474 {
		t.Errorf("freq = %g, want 474", tp.FreqMHz)
	}
	if tp.BandwidthMHz != 8 {
		t.Errorf("bw = %d, want 8", tp.BandwidthMHz)
	}
	if tp.Modulation != "64qam" {
		t.Errorf("modulation = %s, want 64qam", tp.Modulation)
	}
}

func TestParseTransponderEntryDVBS(t *testing.T) {
	fields := map[string]string{
		"DELIVERY_SYSTEM": "DVBS2",
		"FREQUENCY":       "10714000",
		"SYMBOL_RATE":     "22000000",
		"POLARIZATION":    "HORIZONTAL",
		"MODULATION":      "QPSK",
	}
	tp, ok := parseTransponderEntry(fields)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if tp.System != "dvbs2" {
		t.Errorf("system = %s, want dvbs2", tp.System)
	}
	if tp.FreqMHz != 10714 {
		t.Errorf("freq = %g, want 10714", tp.FreqMHz)
	}
	if tp.SymbolRateKS != 22000 {
		t.Errorf("symbol rate = %d, want 22000", tp.SymbolRateKS)
	}
	if tp.Polarization != "h" {
		t.Errorf("polarization = %s, want h", tp.Polarization)
	}
}

func TestParseTransponderEntryDVBC(t *testing.T) {
	fields := map[string]string{
		"DELIVERY_SYSTEM": "DVBC_ANNEX_A",
		"FREQUENCY":       "114000000",
		"SYMBOL_RATE":     "6952000",
		"MODULATION":      "QAM/256",
	}
	tp, ok := parseTransponderEntry(fields)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if tp.System != "dvbc" {
		t.Errorf("system = %s, want dvbc", tp.System)
	}
	if tp.FreqMHz != 114 {
		t.Errorf("freq = %g, want 114", tp.FreqMHz)
	}
	if tp.SymbolRateKS != 6952 {
		t.Errorf("symbol rate = %d, want 6952", tp.SymbolRateKS)
	}
	if tp.Modulation != "256qam" {
		t.Errorf("modulation = %s, want 256qam", tp.Modulation)
	}
}

func TestParseTransponderEntryUnknownSystem(t *testing.T) {
	fields := map[string]string{
		"DELIVERY_SYSTEM": "ATSC",
		"FREQUENCY":       "474000000",
	}
	_, ok := parseTransponderEntry(fields)
	if ok {
		t.Fatal("expected parse to fail for unknown system")
	}
}

func TestParseTransponderEntryMissingFrequency(t *testing.T) {
	fields := map[string]string{
		"DELIVERY_SYSTEM": "DVBT",
	}
	_, ok := parseTransponderEntry(fields)
	if ok {
		t.Fatal("expected parse to fail for missing frequency")
	}
}

func TestParseTransponderEntryDefaultBandwidth(t *testing.T) {
	fields := map[string]string{
		"DELIVERY_SYSTEM": "DVBT",
		"FREQUENCY":       "474000000",
	}
	tp, ok := parseTransponderEntry(fields)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if tp.BandwidthMHz != 8 {
		t.Errorf("default bandwidth = %d, want 8", tp.BandwidthMHz)
	}
}

func TestParseTransponderEntryDefaultModulation(t *testing.T) {
	tests := []struct {
		system string
		want   string
	}{
		{"DVBT", "64qam"},
		{"DVBT2", "256qam"},
		{"DVBS", "qpsk"},
		{"DVBC", "256qam"},
	}
	for _, tt := range tests {
		fields := map[string]string{
			"DELIVERY_SYSTEM": tt.system,
			"FREQUENCY":       "474000000",
		}
		if tt.system == "DVBS" || tt.system == "DVBS2" {
			fields["FREQUENCY"] = "10714000"
		}
		tp, ok := parseTransponderEntry(fields)
		if !ok {
			t.Fatalf("expected successful parse for %s", tt.system)
		}
		if tp.Modulation != tt.want {
			t.Errorf("default modulation for %s = %s, want %s", tt.system, tp.Modulation, tt.want)
		}
	}
}

func TestParseTunerSDP(t *testing.T) {
	sdp := "v=0\r\n" +
		"o=- 0 0 IN IP4 192.168.1.50\r\n" +
		"s=SatIPServer:1 1\r\n" +
		"b=AS:4500\r\n" +
		"a=sendonly\r\n" +
		"a=fmtp:33 tuner=1,200,1,12,474.0,8,dvbt2,256qam,3\r\n"

	info := parseTunerSDP(sdp)
	if info == nil {
		t.Fatal("expected non-nil SignalInfo")
	}
	if info.FeID != 1 {
		t.Errorf("FeID = %d, want 1", info.FeID)
	}
	if info.Level != 200 {
		t.Errorf("Level = %d, want 200", info.Level)
	}
	if !info.Lock {
		t.Error("expected Lock to be true")
	}
	if info.Quality != 12 {
		t.Errorf("Quality = %d, want 12", info.Quality)
	}
	if info.FreqMHz != 474.0 {
		t.Errorf("FreqMHz = %g, want 474", info.FreqMHz)
	}
	if info.BwMHz != 8 {
		t.Errorf("BwMHz = %d, want 8", info.BwMHz)
	}
	if info.Msys != "dvbt2" {
		t.Errorf("Msys = %s, want dvbt2", info.Msys)
	}
	if info.Mtype != "256qam" {
		t.Errorf("Mtype = %s, want 256qam", info.Mtype)
	}
	if info.BitratKbps != 4500 {
		t.Errorf("BitratKbps = %d, want 4500", info.BitratKbps)
	}
	if !info.Active {
		t.Error("expected Active to be true")
	}
}

func TestParseTunerSDPNoTuner(t *testing.T) {
	sdp := "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=No tuner\r\n"
	info := parseTunerSDP(sdp)
	if info != nil {
		t.Error("expected nil for SDP without tuner line")
	}
}
