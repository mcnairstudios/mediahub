package scan

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var DVBTablesDir = resolveDVBTablesDir()

func resolveDVBTablesDir() string {
	if v := os.Getenv("MEDIAHUB_DVB_TABLES_DIR"); v != "" {
		return v
	}
	// Check working directory (project-local dvb/ folder)
	if info, err := os.Stat("dvb"); err == nil && info.IsDir() {
		if abs, err := filepath.Abs("dvb"); err == nil {
			return abs
		}
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		homeDir := filepath.Join(home, "dvb")
		if info, err := os.Stat(homeDir); err == nil && info.IsDir() {
			return homeDir
		}
	}
	return "/usr/share/dvb"
}

func ListTransmitters(system string) ([]string, error) {
	dir := filepath.Join(DVBTablesDir, system)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading transmitter directory %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func ParseTransmitterFile(transmitterFile string) ([]Transponder, error) {
	path := filepath.Join(DVBTablesDir, transmitterFile)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening transmitter file %s: %w", path, err)
	}
	defer f.Close()

	var transponders []Transponder
	current := map[string]string{}
	inBlock := false

	flush := func() {
		if !inBlock || len(current) == 0 {
			return
		}
		tp, ok := parseTransponderEntry(current)
		if ok {
			transponders = append(transponders, tp)
		}
		current = map[string]string{}
		inBlock = false
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			flush()
			inBlock = true
			continue
		}
		if inBlock {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				current[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading transmitter file: %w", err)
	}
	return transponders, nil
}

func parseTransponderEntry(fields map[string]string) (Transponder, bool) {
	system := normaliseSystem(fields["DELIVERY_SYSTEM"])
	if system == "" {
		return Transponder{}, false
	}

	freqStr := fields["FREQUENCY"]
	freqHz, err := strconv.ParseFloat(freqStr, 64)
	if err != nil {
		return Transponder{}, false
	}

	tp := Transponder{
		System:     system,
		Modulation: normaliseModulation(fields["MODULATION"]),
	}

	switch system {
	case "dvbt", "dvbt2":
		tp.FreqMHz = freqHz / 1e6
		bwStr := fields["BANDWIDTH_HZ"]
		if bw, err := strconv.ParseFloat(bwStr, 64); err == nil {
			tp.BandwidthMHz = int(bw / 1e6)
		}
		if tp.BandwidthMHz == 0 {
			tp.BandwidthMHz = 8
		}
		if tp.Modulation == "" {
			if system == "dvbt2" {
				tp.Modulation = "256qam"
			} else {
				tp.Modulation = "64qam"
			}
		}
		if system == "dvbt2" {
			if sid, err := strconv.Atoi(fields["STREAM_ID"]); err == nil && sid >= 0 {
				tp.PLPID = sid
			}
		}
	case "dvbs", "dvbs2":
		tp.FreqMHz = freqHz / 1e3
		if sr, err := strconv.ParseFloat(fields["SYMBOL_RATE"], 64); err == nil {
			tp.SymbolRateKS = int(sr / 1e3)
		}
		tp.Polarization = normalisePolarization(fields["POLARIZATION"])
		tp.FEC = normaliseFEC(fields["INNER_FEC"])
		if tp.Modulation == "" {
			tp.Modulation = "qpsk"
		}
	case "dvbc", "dvbc2":
		tp.FreqMHz = freqHz / 1e6
		if sr, err := strconv.ParseFloat(fields["SYMBOL_RATE"], 64); err == nil {
			tp.SymbolRateKS = int(sr / 1e3)
		}
		if tp.Modulation == "" {
			tp.Modulation = "256qam"
		}
	}

	return tp, true
}

func normaliseSystem(s string) string {
	switch strings.ToUpper(s) {
	case "DVBT":
		return "dvbt"
	case "DVBT2":
		return "dvbt2"
	case "DVBS":
		return "dvbs"
	case "DVBS2":
		return "dvbs2"
	case "DVBC_ANNEX_A", "DVBC_ANNEX_B", "DVBC":
		return "dvbc"
	default:
		return ""
	}
}

func normaliseModulation(s string) string {
	switch strings.ToUpper(s) {
	case "QAM/256", "QAM256":
		return "256qam"
	case "QAM/64", "QAM64":
		return "64qam"
	case "QAM/32", "QAM32":
		return "32qam"
	case "QAM/16", "QAM16":
		return "16qam"
	case "QPSK":
		return "qpsk"
	case "PSK/8", "8PSK":
		return "8psk"
	case "16APSK", "APSK/16":
		return "16apsk"
	case "32APSK", "APSK/32":
		return "32apsk"
	case "8VSB":
		return "8vsb"
	default:
		return ""
	}
}

func normaliseFEC(s string) string {
	s = strings.TrimSpace(s)
	switch s {
	case "1/2":
		return "12"
	case "2/3":
		return "23"
	case "3/4":
		return "34"
	case "3/5":
		return "35"
	case "4/5":
		return "45"
	case "5/6":
		return "56"
	case "7/8":
		return "78"
	case "8/9":
		return "89"
	case "9/10":
		return "910"
	default:
		return ""
	}
}

func normalisePolarization(s string) string {
	switch strings.ToUpper(s) {
	case "HORIZONTAL", "H":
		return "h"
	case "VERTICAL", "V":
		return "v"
	default:
		return ""
	}
}
