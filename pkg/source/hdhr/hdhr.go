package hdhr

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type Device struct {
	Host            string `json:"host"`
	DeviceID        string `json:"device_id"`
	Model           string `json:"model,omitempty"`
	FirmwareVersion string `json:"firmware_version,omitempty"`
	TunerCount      int    `json:"tuner_count,omitempty"`
}

type Config struct {
	ID          string
	Name        string
	IsEnabled   bool
	Devices     []Device
	StreamStore store.StreamStore
	HTTPClient  *http.Client
	discoverer  func() ([]string, error)
}

type Source struct {
	cfg           Config
	streamCount   int
	lastRefreshed *time.Time
	lastError     string
	mu            sync.RWMutex
}

func New(cfg Config) *Source {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.discoverer == nil {
		cfg.discoverer = udpDiscoverHDHR
	}
	return &Source{cfg: cfg}
}

func (s *Source) Type() source.SourceType {
	return "hdhr"
}

func (s *Source) Info(_ context.Context) source.SourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tunerCount := 0
	for _, d := range s.cfg.Devices {
		tunerCount += d.TunerCount
	}

	return source.SourceInfo{
		ID:                  s.cfg.ID,
		Type:                "hdhr",
		Name:                s.cfg.Name,
		IsEnabled:           s.cfg.IsEnabled,
		StreamCount:         s.streamCount,
		LastRefreshed:       s.lastRefreshed,
		LastError:           s.lastError,
		MaxConcurrentStreams: tunerCount,
	}
}

func (s *Source) Refresh(ctx context.Context) error {
	if len(s.cfg.Devices) == 0 {
		s.mu.Lock()
		s.lastError = "no devices configured"
		s.mu.Unlock()
		return fmt.Errorf("no devices configured")
	}

	var allStreams []media.Stream
	var allKeepIDs []string

	for _, device := range s.cfg.Devices {
		baseURL := normalizeHost(device.Host)

		discover, err := s.fetchDiscover(ctx, baseURL)
		if err != nil {
			s.mu.Lock()
			s.lastError = err.Error()
			s.mu.Unlock()
			return fmt.Errorf("discover %s: %w", device.Host, err)
		}

		lineupURL := baseURL + "/lineup.json"
		if discover.LineupURL != "" {
			lineupURL = discover.LineupURL
		}

		lineup, err := s.fetchLineup(ctx, lineupURL)
		if err != nil {
			s.mu.Lock()
			s.lastError = err.Error()
			s.mu.Unlock()
			return fmt.Errorf("lineup %s: %w", device.Host, err)
		}

		for _, entry := range lineup {
			if entry.URL == "" || entry.DRM == 1 {
				continue
			}

			id := deterministicStreamID(s.cfg.ID, entry.GuideNumber)
			allKeepIDs = append(allKeepIDs, id)

			group := classifyGroup(entry)
			vcodec, acodec := normalizeCodecs(entry)

			allStreams = append(allStreams, media.Stream{
				ID:         id,
				SourceType: "hdhr",
				SourceID:   s.cfg.ID,
				Name:       entry.GuideName,
				URL:        entry.URL,
				Group:      group,
				TvgID:      entry.GuideNumber,
				IsActive:   true,
				VideoCodec: vcodec,
				AudioCodec: acodec,
			})
		}
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, allStreams); err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("upserting streams: %w", err)
	}

	if _, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, "hdhr", s.cfg.ID, allKeepIDs); err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("deleting stale streams: %w", err)
	}

	s.mu.Lock()
	s.streamCount = len(allStreams)
	now := time.Now()
	s.lastRefreshed = &now
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, "hdhr", s.cfg.ID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(streams))
	for i, stream := range streams {
		ids[i] = stream.ID
	}
	return ids, nil
}

func (s *Source) DeleteStreams(ctx context.Context) error {
	return s.cfg.StreamStore.DeleteBySource(ctx, "hdhr", s.cfg.ID)
}

func (s *Source) Clear(ctx context.Context) error {
	if err := s.cfg.StreamStore.DeleteBySource(ctx, "hdhr", s.cfg.ID); err != nil {
		return err
	}

	s.mu.Lock()
	s.streamCount = 0
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

func (s *Source) Discover(ctx context.Context) ([]source.DiscoveredDevice, error) {
	ips, err := s.cfg.discoverer()
	if err != nil {
		return nil, err
	}

	existingDeviceIDs := make(map[string]bool)
	for _, d := range s.cfg.Devices {
		existingDeviceIDs[d.DeviceID] = true
	}

	var results []source.DiscoveredDevice
	for _, ip := range ips {
		baseURL := normalizeHost(ip)
		discover, err := s.fetchDiscover(ctx, baseURL)
		if err != nil {
			continue
		}
		results = append(results, source.DiscoveredDevice{
			Host:         ip,
			Identifier:   discover.DeviceID,
			Name:         discover.FriendlyName,
			Model:        discover.ModelNumber,
			AlreadyAdded: existingDeviceIDs[discover.DeviceID],
			Properties: map[string]any{
				"tuner_count":      discover.TunerCount,
				"firmware_version": discover.FirmwareVersion,
			},
		})
	}
	return results, nil
}

func (s *Source) Retune(ctx context.Context) error {
	if len(s.cfg.Devices) == 0 {
		return fmt.Errorf("no devices configured")
	}

	device := s.cfg.Devices[0]
	baseURL := normalizeHost(device.Host)

	scanURL := baseURL + "/lineup.post?scan=start&source=Antenna"
	req, err := http.NewRequestWithContext(ctx, "POST", scanURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("starting scan: %w", err)
	}
	resp.Body.Close()

	for i := 0; i < 600; i++ {
		time.Sleep(2 * time.Second)
		statusReq, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/lineup_status.json", nil)
		if err != nil {
			continue
		}
		statusResp, err := s.cfg.HTTPClient.Do(statusReq)
		if err != nil {
			continue
		}
		var status struct {
			ScanInProgress int `json:"ScanInProgress"`
			Progress       int `json:"Progress"`
			Found          int `json:"Found"`
		}
		json.NewDecoder(statusResp.Body).Decode(&status)
		statusResp.Body.Close()

		if status.ScanInProgress == 0 {
			break
		}
	}

	return s.Refresh(ctx)
}

type discoverResponse struct {
	FriendlyName    string `json:"FriendlyName"`
	ModelNumber     string `json:"ModelNumber"`
	FirmwareVersion string `json:"FirmwareVersion"`
	FirmwareName    string `json:"FirmwareName"`
	DeviceID        string `json:"DeviceID"`
	TunerCount      int    `json:"TunerCount"`
	BaseURL         string `json:"BaseURL"`
	LineupURL       string `json:"LineupURL"`
}

type lineupEntry struct {
	GuideNumber string `json:"GuideNumber"`
	GuideName   string `json:"GuideName"`
	VideoCodec  string `json:"VideoCodec"`
	AudioCodec  string `json:"AudioCodec"`
	URL         string `json:"URL"`
	HD          int    `json:"HD"`
	Favorite    int    `json:"Favorite"`
	DRM         int    `json:"DRM"`
}

func (s *Source) fetchDiscover(ctx context.Context, baseURL string) (*discoverResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/discover.json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to device: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("device returned %d", resp.StatusCode)
	}
	var d discoverResponse
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, fmt.Errorf("parsing discover response: %w", err)
	}
	return &d, nil
}

func (s *Source) fetchLineup(ctx context.Context, lineupURL string) ([]lineupEntry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", lineupURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching lineup: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("lineup returned %d", resp.StatusCode)
	}
	var entries []lineupEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("parsing lineup: %w", err)
	}
	return entries, nil
}

func classifyGroup(entry lineupEntry) string {
	vcodec := strings.ToLower(entry.VideoCodec)
	if vcodec == "" || vcodec == "none" {
		return "Radio"
	}
	if entry.HD == 1 {
		return "HD"
	}
	return "SD"
}

func normalizeCodecs(entry lineupEntry) (video, audio string) {
	vcodec := strings.ToLower(entry.VideoCodec)
	switch vcodec {
	case "mpeg2":
		video = "mpeg2video"
	case "h264":
		video = "h264"
	case "hevc", "h265":
		video = "hevc"
	case "", "none":
		video = ""
	default:
		video = vcodec
	}

	acodec := strings.ToLower(entry.AudioCodec)
	switch acodec {
	case "mpeg", "mp2":
		audio = "mp2"
	case "":
		audio = "aac"
	default:
		audio = acodec
	}

	return video, audio
}

func deterministicStreamID(sourceID, guideNumber string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + guideNumber))
	return fmt.Sprintf("%x", h[:16])
}

func normalizeHost(host string) string {
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/")
	}
	return "http://" + strings.TrimRight(host, "/")
}

var hdhrDiscoverPacket = []byte{
	0x00, 0x02, 0x00, 0x0c,
	0x01, 0x04, 0xff, 0xff, 0xff, 0xff,
	0x02, 0x04, 0xff, 0xff, 0xff, 0xff,
	0x73, 0xcc, 0x7d, 0x8f,
}

func udpDiscoverHDHR() ([]string, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}
	defer conn.Close()

	dst := &net.UDPAddr{IP: net.IPv4(255, 255, 255, 255), Port: 65001}
	if _, err := conn.WriteTo(hdhrDiscoverPacket, dst); err != nil {
		return nil, fmt.Errorf("broadcast: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	seen := make(map[string]bool)
	buf := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			break
		}
		if n < 2 || buf[1] != 0x03 {
			continue
		}
		ip := addr.(*net.UDPAddr).IP.String()
		seen[ip] = true
	}

	var ips []string
	for ip := range seen {
		ips = append(ips, ip)
	}
	return ips, nil
}
