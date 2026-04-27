package hdhr

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

const (
	DefaultDeviceID        = "12345678"
	DefaultFriendlyName    = "MediaHub"
	DefaultModelNumber     = "HDHR5-4US"
	DefaultFirmwareName    = "hdhomerun5_universal"
	DefaultFirmwareVersion = "20231001"
	DefaultDeviceAuth      = "mediahub"
	DefaultTunerCount      = 6
)

type Server struct {
	mux          *http.ServeMux
	channelStore channel.Store
	cfg          *config.Config
}

func NewServer(channelStore channel.Store, cfg *config.Config) *Server {
	s := &Server{
		mux:          http.NewServeMux(),
		channelStore: channelStore,
		cfg:          cfg,
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /discover.json", s.handleDiscover)
	s.mux.HandleFunc("GET /lineup_status.json", s.handleLineupStatus)
	s.mux.HandleFunc("GET /lineup.json", s.handleLineup)
	s.mux.HandleFunc("GET /lineup.xml", s.handleLineupXML)
	s.mux.HandleFunc("GET /device.xml", s.handleDeviceXML)
}

func (s *Server) baseURL() string {
	return s.cfg.BaseURL
}

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL()
	resp := DiscoverResponse{
		FriendlyName:    DefaultFriendlyName,
		Manufacturer:    "Silicondust",
		ManufacturerURL: "https://www.silicondust.com/",
		ModelNumber:     DefaultModelNumber,
		FirmwareName:    DefaultFirmwareName,
		FirmwareVersion: DefaultFirmwareVersion,
		DeviceID:        DefaultDeviceID,
		DeviceAuth:      DefaultDeviceAuth,
		BaseURL:         base,
		LineupURL:       base + "/lineup.json",
		TunerCount:      DefaultTunerCount,
	}
	httputil.RespondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLineupStatus(w http.ResponseWriter, r *http.Request) {
	resp := LineupStatus{
		ScanInProgress: 0,
		ScanPossible:   1,
		Source:         "Cable",
		SourceList:     []string{"Cable", "Antenna"},
	}
	httputil.RespondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLineup(w http.ResponseWriter, r *http.Request) {
	lineup, err := s.buildLineup(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to build lineup")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, lineup)
}

func (s *Server) handleLineupXML(w http.ResponseWriter, r *http.Request) {
	lineup, err := s.buildLineup(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to build lineup")
		return
	}

	type xmlLineup struct {
		XMLName xml.Name      `xml:"Lineup"`
		Items   []LineupEntry `xml:"Program"`
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	xml.NewEncoder(w).Encode(xmlLineup{Items: lineup})
}

func (s *Server) handleDeviceXML(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL()
	resp := DeviceXML{
		XMLNS:   "urn:schemas-upnp-org:device-1-0",
		URLBase: base,
		SpecVersion: specVersionXML{
			Major: 1,
			Minor: 0,
		},
		Device: deviceInnerXML{
			DeviceType:   "urn:schemas-upnp-org:device:MediaServer:1",
			FriendlyName: DefaultFriendlyName,
			Manufacturer: "Silicondust",
			ModelName:    DefaultModelNumber,
			ModelNumber:  DefaultModelNumber,
			SerialNumber: DefaultDeviceID,
			UDN:          "uuid:" + DefaultDeviceID,
		},
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	xml.NewEncoder(w).Encode(resp)
}

func (s *Server) buildLineup(ctx context.Context) ([]LineupEntry, error) {
	channels, err := s.channelStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing channels: %w", err)
	}

	base := s.baseURL()
	lineup := make([]LineupEntry, 0, len(channels))
	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		lineup = append(lineup, LineupEntry{
			GuideNumber: strconv.Itoa(ch.Number),
			GuideName:   ch.Name,
			VideoCodec:  "H264",
			AudioCodec:  "AAC",
			HD:          1,
			URL:         fmt.Sprintf("%s/channel/%s", base, ch.ID),
		})
	}
	return lineup, nil
}
