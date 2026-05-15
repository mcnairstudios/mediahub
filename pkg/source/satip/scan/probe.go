package scan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	astits "github.com/asticode/go-astits"
	"github.com/rs/zerolog"
)

type scanResult struct {
	tp          Transponder
	channels    []Channel
	nitMuxes    []Transponder
	networkID   uint16
	networkName string
	elapsed     time.Duration
	err         error
	patReceived bool
	nitComplete bool
	signalOnly  bool
	pmtData     map[uint16]*astits.PMTData
	programs    map[uint16]uint16
}

type SingleResult struct {
	Channels    []Channel
	NITMuxes    []Transponder
	NetworkID   uint16
	NetworkName string
	Err         error
}

func ScanSingleTransponder(host string, tp Transponder, timeout time.Duration, log zerolog.Logger) SingleResult {
	return ScanTransponderWithPids(host, tp, timeout, "all", log)
}

func ScanTransponderWithPids(host string, tp Transponder, timeout time.Duration, pids string, log zerolog.Logger) SingleResult {
	r := scanTransponder(context.Background(), host, tp, timeout, pids, log)
	return SingleResult{
		Channels:    r.channels,
		NITMuxes:    r.nitMuxes,
		NetworkID:   r.networkID,
		NetworkName: r.networkName,
		Err:         r.err,
	}
}

// parseResult holds parsed transport stream data from the two-pass demuxer.
type parseResult struct {
	programs      map[uint16]uint16 // program_number -> PMT PID
	serviceInfo   map[uint16]sdtInfo
	pmtData       map[uint16]pmtInfo
	discoveredMuxes []Transponder
	networkID     uint16
	networkName   string
	nitComplete   bool
}

type sdtInfo struct {
	serviceName  string
	providerName string
	serviceType  uint8
	encrypted    bool
}

type pmtInfo struct {
	pcrPID  uint16
	streams []StreamComponent
}

// filterTSPIDs extracts TS packets matching the given PIDs from raw TS data.
// If pids is nil, all packets are passed through.
func filterTSPIDs(data []byte, pids map[uint16]bool) []byte {
	if pids == nil || len(data) < 188 {
		return data
	}
	out := make([]byte, 0, len(data)/4) // PSI is a fraction of total
	for i := 0; i+188 <= len(data); i += 188 {
		if data[i] != 0x47 {
			continue
		}
		pid := uint16(data[i+1]&0x1F)<<8 | uint16(data[i+2])
		if pids[pid] {
			out = append(out, data[i:i+188]...)
		}
	}
	return out
}

// demuxTS parses the given TS data with astits, collecting PAT, SDT, NIT, and PMT.
// It does two passes: first to get PAT/SDT/NIT (which reveals PMT PIDs), then PMT.
func demuxTS(data []byte, dvbType string, log zerolog.Logger) parseResult {
	r := parseResult{
		programs:    make(map[uint16]uint16),
		serviceInfo: make(map[uint16]sdtInfo),
		pmtData:     make(map[uint16]pmtInfo),
	}

	// Pass 1: filter to well-known PIDs (PAT=0, NIT=16, SDT=17)
	// and parse to discover PMT PIDs from PAT
	knownPIDs := map[uint16]bool{0: true, 16: true, 17: true}
	filtered := filterTSPIDs(data, knownPIDs)
	demuxPass1(filtered, &r, dvbType, log)

	// Pass 2: if we found PMT PIDs, filter and parse those too
	// Include PAT (PID 0) so the demuxer learns which PIDs carry PMT
	if len(r.programs) > 0 {
		pmtPIDs := map[uint16]bool{0: true} // PAT needed for programMap
		for _, pmtPID := range r.programs {
			if pmtPID > 0 {
				pmtPIDs[pmtPID] = true
			}
		}
		pmtFiltered := filterTSPIDs(data, pmtPIDs)
		if len(pmtFiltered) > 0 {
			demuxPMT(pmtFiltered, &r)
		}
	}

	return r
}

func demuxPass1(data []byte, r *parseResult, dvbType string, log zerolog.Logger) {
	reader := bytes.NewReader(data)
	demuxer := astits.NewDemuxer(context.Background(), reader)

	consecutiveErrors := 0
	for {
		d, err := demuxer.NextData()
		if err != nil {
			consecutiveErrors++
			if errors.Is(err, astits.ErrNoMorePackets) || consecutiveErrors > 50 {
				break
			}
			errStr := err.Error()
			if strings.Contains(errStr, "EOF") || strings.Contains(errStr, "closed pipe") {
				break
			}
			continue
		}
		consecutiveErrors = 0

		if d.PAT != nil {
			for _, prog := range d.PAT.Programs {
				if prog.ProgramNumber != 0 {
					r.programs[prog.ProgramNumber] = prog.ProgramMapID
				}
			}
		}

		if d.SDT != nil {
			for _, svc := range d.SDT.Services {
				info := sdtInfo{
					encrypted: svc.HasFreeCSAMode,
				}
				for _, desc := range svc.Descriptors {
					if desc.Tag == astits.DescriptorTagService && desc.Service != nil {
						info.serviceType = desc.Service.Type
						info.serviceName = dvbString(desc.Service.Name)
						info.providerName = dvbString(desc.Service.Provider)
					}
				}
				r.serviceInfo[svc.ServiceID] = info
			}
		}

		if d.NIT != nil {
			if r.networkID == 0 {
				r.networkID = d.NIT.NetworkID
				for _, desc := range d.NIT.NetworkDescriptors {
					if desc.NetworkName != nil {
						r.networkName = dvbString(desc.NetworkName.Name)
						break
					}
				}
			}
			for _, ts := range d.NIT.TransportStreams {
				mux := extractNITTransponder(ts.TransportDescriptors, dvbType)
				if mux != nil {
					r.discoveredMuxes = append(r.discoveredMuxes, *mux)
				}
			}
			r.nitComplete = true
		}
	}
}

// demuxPMT parses PMT sections from pre-filtered TS data containing only PMT PIDs.
func demuxPMT(data []byte, r *parseResult) {
	reader := bytes.NewReader(data)
	demuxer := astits.NewDemuxer(context.Background(), reader)

	consecutiveErrors := 0
	for {
		d, err := demuxer.NextData()
		if err != nil {
			consecutiveErrors++
			if errors.Is(err, astits.ErrNoMorePackets) || consecutiveErrors > 50 {
				break
			}
			errStr := err.Error()
			if strings.Contains(errStr, "EOF") || strings.Contains(errStr, "closed pipe") {
				break
			}
			continue
		}
		consecutiveErrors = 0

		if d.PMT != nil {
			info := pmtInfo{
				pcrPID: d.PMT.PCRPID,
			}
			for _, es := range d.PMT.ElementaryStreams {
				comp := StreamComponent{
					PID:        es.ElementaryPID,
					StreamType: uint8(es.StreamType),
					TypeName:   streamTypStr(uint8(es.StreamType)),
					Category:   streamCategory(uint8(es.StreamType)),
				}
				for _, desc := range es.ElementaryStreamDescriptors {
					if desc.ISO639LanguageAndAudioType != nil {
						comp.Language = strings.TrimRight(string(desc.ISO639LanguageAndAudioType.Language[:]), "\x00")
						comp.AudioType = desc.ISO639LanguageAndAudioType.Type
						if comp.Category == "" {
							comp.Category = "audio"
						}
					}
					if desc.Component != nil && len(desc.Component.Text) > 0 {
						comp.Label = dvbString(desc.Component.Text)
					}
					if desc.Subtitling != nil {
						comp.Category = "subtitle"
						if len(desc.Subtitling.Items) > 0 && len(desc.Subtitling.Items[0].Language) > 0 {
							comp.Language = strings.TrimRight(string(desc.Subtitling.Items[0].Language[:]), "\x00")
						}
					}
					if desc.Teletext != nil || desc.VBITeletext != nil {
						comp.Category = "teletext"
					}
				}
				info.streams = append(info.streams, comp)
			}
			r.pmtData[d.PMT.ProgramNumber] = info
		}
	}
}

// extractNITTransponder extracts a Transponder from NIT transport stream descriptors
// using the typed astits delivery system descriptor API (requires go-astits fork).
func extractNITTransponder(descs []*astits.Descriptor, dvbType string) *Transponder {
	for _, desc := range descs {
		switch {
		case desc.Tag == astits.DescriptorTagSatelliteDeliverySystem && isSatType(dvbType) && desc.SatelliteDeliverySystem != nil:
			sd := desc.SatelliteDeliverySystem
			// Frequency is BCD-decoded in 10 kHz units; divide by 100 for MHz
			freqMHz := float64(sd.Frequency) / 100.0
			if freqMHz < 1.0 {
				continue
			}

			var pol string
			switch sd.Polarization {
			case 0:
				pol = "h"
			case 1:
				pol = "v"
			case 2:
				pol = "l"
			case 3:
				pol = "r"
			}

			deliverySys := "dvbs"
			if sd.ModulationSystem {
				deliverySys = "dvbs2"
			}

			var mod string
			switch sd.ModulationType {
			case 0, 1:
				mod = "qpsk"
			case 2:
				mod = "8psk"
			case 3:
				mod = "16apsk"
			}

			// SymbolRate is BCD-decoded in 100 sym/s units; divide by 10 for ksym/s
			symbolRateKS := int(sd.SymbolRate) / 10
			if symbolRateKS < 1000 || symbolRateKS > 50000 {
				continue
			}

			fec := fecInnerToSATIP(sd.FECInner)

			return &Transponder{
				FreqMHz:      freqMHz,
				System:       deliverySys,
				Modulation:   mod,
				SymbolRateKS: symbolRateKS,
				Polarization: pol,
				FEC:          fec,
			}

		case desc.Tag == astits.DescriptorTagTerrestrialDeliverySystem && isTerrestrialType(dvbType) && desc.TerrestrialDeliverySystem != nil:
			td := desc.TerrestrialDeliverySystem
			// Frequency is in 10 Hz units
			freqHz := int(td.Frequency) * 10
			freqMHz := float64(freqHz) / 1000000.0
			if freqMHz < 50.0 || freqMHz > 900.0 {
				continue
			}

			bwMHz := bandwidthToMHz(td.Bandwidth)

			return &Transponder{
				FreqMHz:      freqMHz,
				System:       "dvbt",
				Modulation:   "64qam",
				BandwidthMHz: bwMHz,
			}

		case desc.Tag == astits.DescriptorTagExtension && isTerrestrialType(dvbType) && desc.Extension != nil:
			if desc.Extension.Tag == astits.DescriptorTagExtensionT2DeliverySystem && desc.Extension.T2DeliverySystem != nil {
				t2 := desc.Extension.T2DeliverySystem
				bwMHz := bandwidthToMHz(t2.Bandwidth)

				var freqMHz float64
				if len(t2.Cells) > 0 && len(t2.Cells[0].CentreFrequencies) > 0 {
					freqHz := int(t2.Cells[0].CentreFrequencies[0]) * 10
					freqMHz = float64(freqHz) / 1000000.0
				}
				if freqMHz < 50.0 || freqMHz > 900.0 {
					continue
				}

				return &Transponder{
					FreqMHz:      freqMHz,
					System:       "dvbt2",
					Modulation:   "256qam",
					BandwidthMHz: bwMHz,
					PLPID:        int(t2.PLPID),
				}
			}

		// Fallback: parse raw bytes for devices that don't use typed descriptors
		case desc.Unknown != nil:
			switch desc.Unknown.Tag {
			case tagTerrestrialDelivery:
				if isTerrestrialType(dvbType) {
					if mux, ok := parseTerrestrialDelivery(desc.Unknown.Content); ok {
						return &mux
					}
				}
			case tagSatelliteDelivery:
				if isSatType(dvbType) {
					if mux, ok := parseSatelliteDelivery(desc.Unknown.Content); ok {
						return &mux
					}
				}
			case tagCableDelivery:
				if mux, ok := parseCableDelivery(desc.Unknown.Content); ok {
					return &mux
				}
			}
		case desc.Extension != nil && desc.Extension.Tag == tagExtT2Delivery && desc.Extension.Unknown != nil:
			if isTerrestrialType(dvbType) {
				if mux, ok := parseT2Delivery(*desc.Extension.Unknown); ok {
					return &mux
				}
			}
		}
	}
	return nil
}

func isSatType(dvbType string) bool {
	return dvbType == "dvbs" || dvbType == "dvbs2"
}

func isTerrestrialType(dvbType string) bool {
	return dvbType == "dvbt" || dvbType == "dvbt2"
}

func fecInnerToSATIP(fec uint8) string {
	switch fec {
	case 1:
		return "12"
	case 2:
		return "23"
	case 3:
		return "34"
	case 4:
		return "56"
	case 5:
		return "78"
	case 6:
		return "89"
	case 7:
		return "35"
	case 8:
		return "45"
	case 9:
		return "910"
	default:
		return ""
	}
}

func bandwidthToMHz(bw uint8) int {
	switch bw {
	case 0:
		return 8
	case 1:
		return 7
	case 2:
		return 6
	case 3:
		return 5
	case 4:
		return 10
	case 5:
		return 2 // 1.712 MHz
	default:
		return 8
	}
}

// dvbType infers the DVB type from a transponder's system field.
func dvbTypeFromSystem(sys string) string {
	switch sys {
	case "dvbs", "dvbs2":
		return "dvbs"
	case "dvbt", "dvbt2":
		return "dvbt"
	case "dvbc", "dvbc2":
		return "dvbc"
	default:
		return sys
	}
}

// scanTransponder scans a single transponder using the collect-all + two-pass approach.
// It opens an RTSP session, collects raw TS data via UDP, then parses PAT/SDT/NIT/PMT
// client-side with PID filtering.
func scanTransponder(parentCtx context.Context, host string, tp Transponder, timeout time.Duration, pids string, log zerolog.Logger) (result scanResult) {
	start := time.Now()
	result.tp = tp
	defer func() { result.elapsed = time.Since(start) }()

	c, err := dialRTSP(host, 5*time.Second)
	if err != nil {
		result.err = err
		return result
	}
	defer c.close()
	// Set TCP deadline from parent context so hung RTSP operations don't block forever
	if dl, ok := parentCtx.Deadline(); ok {
		c.conn.SetDeadline(dl)
	} else {
		c.conn.SetDeadline(time.Now().Add(15 * time.Second))
	}

	// Use pids=all for data collection — we filter client-side
	rtspURL := tp.RTSPURL(host, "all")
	resp, err := c.send("DESCRIBE", rtspURL, map[string]string{"Accept": "application/sdp"}, nil)
	if err != nil {
		result.err = err
		return result
	}
	log.Debug().Str("method", "DESCRIBE").Int("status", resp.status).Msg("rtsp")

	// Open a UDP socket for RTP data before SETUP.
	clientPort, err := c.listenUDP()
	if err != nil {
		result.err = fmt.Errorf("listen UDP: %w", err)
		return result
	}
	// Set a generous receive buffer for pids=all
	c.udpConn.SetReadBuffer(1024 * 1024) //nolint:errcheck
	transport := fmt.Sprintf("RTP/AVP;unicast;client_port=%d-%d", clientPort, clientPort+1)

	var controlURL, session string

	if resp.status == 200 {
		// Standard SAT>IP path: extract control URL from SDP
		controlURL = fmt.Sprintf("rtsp://%s/stream=1", host)
		for _, line := range strings.Split(string(resp.body), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "a=control:") {
				ctrl := strings.TrimPrefix(line, "a=control:")
				if strings.HasPrefix(ctrl, "rtsp://") {
					controlURL = ctrl
				} else {
					controlURL = fmt.Sprintf("rtsp://%s/%s", host, ctrl)
				}
			}
		}

		session = resp.headers["session"]
		setupHeaders := map[string]string{
			"Transport": transport,
		}
		if session != "" {
			setupHeaders["Session"] = session
		}
		resp, err = c.send("SETUP", controlURL, setupHeaders, nil)
		if err != nil {
			result.err = err
			return result
		}
		if resp.status != 200 {
			result.err = fmt.Errorf("SETUP returned %d", resp.status)
			return result
		}
		if s := resp.headers["session"]; s != "" {
			session = strings.SplitN(s, ";", 2)[0]
		}
	} else if resp.status == 404 {
		// Fallback for devices that don't support DESCRIBE (e.g. Astra Netstream 4SAT).
		// Go straight to SETUP with the tuning parameters in the URL.
		log.Debug().Msg("DESCRIBE returned 404, falling back to direct SETUP")
		resp, err = c.send("SETUP", rtspURL, map[string]string{
			"Transport": transport,
		}, nil)
		if err != nil {
			result.err = err
			return result
		}
		if resp.status != 200 {
			result.err = fmt.Errorf("SETUP returned %d", resp.status)
			return result
		}
		if s := resp.headers["session"]; s != "" {
			session = strings.SplitN(s, ";", 2)[0]
		}
		// Build control URL from stream ID returned by the device
		if streamID := resp.headers["com.ses.streamid"]; streamID != "" {
			controlURL = fmt.Sprintf("rtsp://%s/stream=%s", host, streamID)
		} else {
			controlURL = rtspURL
		}
	} else {
		result.err = fmt.Errorf("DESCRIBE returned %d", resp.status)
		return result
	}

	resp, err = c.send("PLAY", controlURL, map[string]string{
		"Session": session,
		"Range":   "npt=0.000-",
	}, nil)
	if err != nil {
		result.err = err
		return result
	}
	log.Debug().Str("method", "PLAY").Int("status", resp.status).Msg("rtsp")
	if resp.status != 200 {
		c.teardown(controlURL, session)
		result.err = fmt.Errorf("PLAY returned %d", resp.status)
		return result
	}

	// Collect UDP data with fast no-signal detection.
	// Short initial deadline: if no valid TS data arrives in 4s, there's no signal.
	// Once we get TS data, extend to the full timeout.
	tsData := collectRTPData(c.udpConn, timeout, log)
	c.teardown(controlURL, session)

	if len(tsData) == 0 {
		log.Debug().Str("mux", tp.String()).Msg("no data received")
		result.err = fmt.Errorf("no data received (no signal)")
		return result
	}

	// Align on TS sync bytes (0x47 every 188 bytes)
	tsData = alignTS(tsData)
	if len(tsData) == 0 {
		log.Debug().Str("mux", tp.String()).Msg("no valid TS data after alignment")
		result.err = fmt.Errorf("no valid TS data")
		return result
	}

	result.patReceived = true // We got data, so there's signal

	// For signal-only checks, we just need to confirm data arrived
	result.signalOnly = (pids == "0")
	if result.signalOnly {
		return result
	}

	// Determine DVB type from the transponder system
	dvbType := dvbTypeFromSystem(tp.System)

	// Parse transport stream with two-pass demuxer
	parsed := demuxTS(tsData, dvbType, log)
	result.networkID = parsed.networkID
	result.networkName = parsed.networkName
	result.nitComplete = parsed.nitComplete

	// Collect NIT-discovered muxes (deduplication is done by caller)
	result.nitMuxes = parsed.discoveredMuxes

	// Build channels from parsed data
	programs := parsed.programs
	result.programs = programs
	result.pmtData = make(map[uint16]*astits.PMTData)

	for progNum, pmtPID := range programs {
		name := ""
		var svcType uint8
		var encrypted bool
		if info, ok := parsed.serviceInfo[progNum]; ok {
			name = info.serviceName
			svcType = info.serviceType
			encrypted = info.encrypted
		}
		if name == "" {
			name = fmt.Sprintf("SID:%d", progNum)
		}
		ch := Channel{
			Name:        name,
			ServiceID:   progNum,
			ServiceType: svcType,
			Encrypted:   encrypted,
			PMTPID:      pmtPID,
			Transponder: tp,
		}
		if pmt, ok := parsed.pmtData[progNum]; ok {
			ch.PCRPID = pmt.pcrPID
			ch.Streams = pmt.streams
		}
		result.channels = append(result.channels, ch)
	}

	sort.Slice(result.channels, func(i, j int) bool {
		return result.channels[i].ServiceID < result.channels[j].ServiceID
	})

	log.Debug().Str("mux", tp.String()).
		Int("services", len(result.channels)).
		Int("nit_muxes", len(result.nitMuxes)).
		Int("ts_bytes", len(tsData)).
		Msg("scan complete")

	return result
}

// collectRTPData reads RTP packets from a UDP connection, strips headers,
// and returns raw TS data. Uses fast no-signal detection: starts with a
// short 4s deadline, extends to full timeout only after first valid TS packet.
func collectRTPData(conn *net.UDPConn, timeout time.Duration, log zerolog.Logger) []byte {
	// Pre-allocate generously — a 5s scan at ~10 Mbps is ~6 MB
	tsData := make([]byte, 0, 8*1024*1024)
	buf := make([]byte, 65536)

	// Start with short deadline for fast no-signal detection
	conn.SetReadDeadline(time.Now().Add(4 * time.Second)) //nolint:errcheck
	gotTS := false

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			break
		}
		if n < 12 {
			continue
		}

		// Strip RTP header and extract TS payload
		payload, stripErr := stripRTPHeader(buf[:n])
		if stripErr != nil || len(payload) == 0 {
			continue
		}

		// Validate it looks like TS data
		if len(payload)%188 == 0 && payload[0] == 0x47 {
			if !gotTS {
				// First TS data — extend deadline for full collection
				gotTS = true
				conn.SetReadDeadline(time.Now().Add(timeout)) //nolint:errcheck
			}
			tsData = append(tsData, payload...)
		}
	}

	return tsData
}

// alignTS finds the start of aligned TS packets (0x47 sync byte every 188 bytes)
// and returns the aligned data.
func alignTS(data []byte) []byte {
	if len(data) < 188*2 {
		return nil
	}

	// Find first sync byte verified by the next packet boundary
	for i := 0; i < len(data)-188*2; i++ {
		if data[i] == 0x47 && data[i+188] == 0x47 && data[i+376] == 0x47 {
			aligned := data[i:]
			aligned = aligned[:len(aligned)/188*188]
			return aligned
		}
	}
	return nil
}

// shouldNotRetry returns true for errors that won't resolve with a retry:
// device busy (503/453), no signal (timeout/no data), or non-transient RTSP errors.
func shouldNotRetry(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// Device busy — retrying just adds pressure
	if strings.Contains(s, "503") || strings.Contains(s, "453") {
		return true
	}
	// Timeout / no data — no signal on this frequency, retrying won't help
	if strings.Contains(s, "i/o timeout") || strings.Contains(s, "no data") || strings.Contains(s, "no signal") {
		return true
	}
	// SETUP rejected (403 forbidden) — permanent
	if strings.Contains(s, "SETUP returned 403") {
		return true
	}
	return false
}

// scanJob represents a mux to scan, with retry tracking.
type scanJob struct {
	idx     int
	tp      Transponder
	attempt int
}

func scanParallel(host string, tps []Transponder, maxParallel int, timeout time.Duration, pass int, log zerolog.Logger, progressFn func(MuxProgress)) []scanResult {
	if maxParallel < 1 {
		maxParallel = 1
	}

	results := make([]scanResult, len(tps))
	total := len(tps)
	var completed atomic.Int32

	// Per-mux timeout: enough for RTSP setup + data collection + teardown.
	muxTimeout := timeout * 3
	if muxTimeout < 30*time.Second {
		muxTimeout = 30 * time.Second
	}

	// Work queue: tuners pull jobs, failed jobs get re-queued with backoff.
	jobs := make(chan scanJob, len(tps))
	retries := make(chan scanJob, len(tps))

	// Seed the queue
	for i, tp := range tps {
		jobs <- scanJob{idx: i, tp: tp, attempt: 0}
	}
	close(jobs)

	// Backoff schedule: short delays for transient failures (PLAY 404, connection reset)
	backoffs := []time.Duration{3 * time.Second, 10 * time.Second, 30 * time.Second}
	maxAttempts := len(backoffs) + 1 // first attempt + 3 retries

	var wg sync.WaitGroup
	for w := 0; w < maxParallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), muxTimeout)
				results[job.idx] = scanTransponder(ctx, host, job.tp, timeout, "all", log)
				cancel()

				r := results[job.idx]
				// Only retry transient errors (timeout, no data). Don't retry device-busy (503/453).
				retryable := r.err != nil && job.attempt+1 < maxAttempts && !shouldNotRetry(r.err)
				if retryable {
					log.Warn().Err(r.err).Str("mux", job.tp.String()).Int("attempt", job.attempt+1).Msg("mux failed, will retry")
					retries <- scanJob{idx: job.idx, tp: job.tp, attempt: job.attempt + 1}
				} else {
					done := int(completed.Add(1))
					if r.err != nil {
						log.Error().Err(r.err).Str("mux", job.tp.String()).Int("done", done).Int("total", total).Msg("mux failed")
					} else {
						log.Info().Str("mux", job.tp.String()).Int("channels", len(r.channels)).Int("done", done).Int("total", total).Msg("mux scanned")
					}
					if progressFn != nil {
						progressFn(MuxProgress{
							Done:     done,
							Total:    total,
							Pass:     pass,
							Mux:      job.tp,
							Services: r.channels,
							Error:    r.err,
						})
					}
				}
			}
		}()
	}
	wg.Wait()
	close(retries)

	// Process retries with backoff
	for len(retries) > 0 {
		var batch []scanJob
		for job := range retries {
			batch = append(batch, job)
		}
		if len(batch) == 0 {
			break
		}

		backoff := backoffs[0]
		if batch[0].attempt-1 < len(backoffs) {
			backoff = backoffs[batch[0].attempt-1]
		}
		log.Info().Int("retries", len(batch)).Dur("backoff", backoff).Msg("retrying failed muxes")
		time.Sleep(backoff)

		nextRetries := make(chan scanJob, len(batch))
		jobs2 := make(chan scanJob, len(batch))
		for _, job := range batch {
			jobs2 <- job
		}
		close(jobs2)

		var wg2 sync.WaitGroup
		for w := 0; w < maxParallel; w++ {
			wg2.Add(1)
			go func() {
				defer wg2.Done()
				for job := range jobs2 {
					ctx, cancel := context.WithTimeout(context.Background(), muxTimeout)
					results[job.idx] = scanTransponder(ctx, host, job.tp, timeout, "all", log)
					cancel()

					r := results[job.idx]
					retryable := r.err != nil && job.attempt+1 < maxAttempts && !shouldNotRetry(r.err)
					if retryable {
						log.Warn().Err(r.err).Str("mux", job.tp.String()).Int("attempt", job.attempt+1).Msg("mux failed, will retry")
						nextRetries <- scanJob{idx: job.idx, tp: job.tp, attempt: job.attempt + 1}
					} else {
						done := int(completed.Add(1))
						if r.err != nil {
							log.Error().Err(r.err).Str("mux", job.tp.String()).Int("done", done).Int("total", total).Msg("mux failed")
						} else {
							log.Info().Str("mux", job.tp.String()).Int("channels", len(r.channels)).Int("done", done).Int("total", total).Msg("mux scanned")
						}
						if progressFn != nil {
							progressFn(MuxProgress{
								Done:     done,
								Total:    total,
								Pass:     pass,
								Mux:      job.tp,
								Services: r.channels,
								Error:    r.err,
							})
						}
					}
				}
			}()
		}
		wg2.Wait()
		close(nextRetries)

		retries = nextRetries
	}

	return results
}

