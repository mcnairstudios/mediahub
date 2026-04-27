package dlna

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	ssdpMulticastAddr = "239.255.255.250:1900"
	ssdpMaxAge        = 1800
)

type SSDPAdvertiser struct {
	server           *Server
	baseURL          string
	port             int
	log              zerolog.Logger
	announceInterval time.Duration
	conn             *net.UDPConn
	mu               sync.Mutex
}

func NewSSDPAdvertiser(server *Server, baseURL string, port int, announceInterval time.Duration, log zerolog.Logger) *SSDPAdvertiser {
	if announceInterval <= 0 {
		announceInterval = 30 * time.Second
	}
	return &SSDPAdvertiser{
		server:           server,
		baseURL:          baseURL,
		port:             port,
		log:              log.With().Str("component", "dlna-ssdp").Logger(),
		announceInterval: announceInterval,
	}
}

func (a *SSDPAdvertiser) Run(ctx context.Context) {
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return
	}

	listener, err := a.startListener()
	if err != nil {
		a.log.Error().Err(err).Msg("failed to start SSDP listener")
		return
	}
	defer listener.Close()

	a.sendAlive()

	ticker := time.NewTicker(a.announceInterval)
	defer ticker.Stop()

	go a.listenForSearches(ctx, listener)

	for {
		select {
		case <-ctx.Done():
			a.sendByeBye()
			return
		case <-ticker.C:
			if !a.server.settings.IsEnabled(ctx) {
				continue
			}
			a.sendAlive()
		}
	}
}

func (a *SSDPAdvertiser) startListener() (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp4", ssdpMulticastAddr)
	if err != nil {
		return nil, fmt.Errorf("resolving multicast addr: %w", err)
	}
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("listening multicast: %w", err)
	}
	conn.SetReadBuffer(8192)
	return conn, nil
}

func (a *SSDPAdvertiser) listenForSearches(ctx context.Context, conn *net.UDPConn) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			a.log.Debug().Err(err).Msg("SSDP read error")
			continue
		}

		msg := string(buf[:n])
		if !strings.Contains(msg, "M-SEARCH") {
			continue
		}

		if !a.server.settings.IsEnabled(ctx) {
			continue
		}

		st := extractHeader(msg, "ST")
		if st != "ssdp:all" && st != "upnp:rootdevice" && st != "urn:schemas-upnp-org:device:MediaServer:1" {
			continue
		}

		a.sendSearchResponse(remoteAddr, st)
	}
}

func (a *SSDPAdvertiser) sendAlive() {
	location := a.location()
	udn := a.server.UDN()
	usn := udn + "::urn:schemas-upnp-org:device:MediaServer:1"

	msg := "NOTIFY * HTTP/1.1\r\n" +
		"HOST: " + ssdpMulticastAddr + "\r\n" +
		"CACHE-CONTROL: max-age=" + fmt.Sprintf("%d", ssdpMaxAge) + "\r\n" +
		"LOCATION: " + location + "\r\n" +
		"NT: urn:schemas-upnp-org:device:MediaServer:1\r\n" +
		"NTS: ssdp:alive\r\n" +
		"SERVER: MediaHub/1.0 UPnP/1.0 DLNA/1.50\r\n" +
		"USN: " + usn + "\r\n" +
		"\r\n"

	a.sendMulticast([]byte(msg))
	a.log.Debug().Str("location", location).Msg("SSDP alive sent")
}

func (a *SSDPAdvertiser) sendByeBye() {
	udn := a.server.UDN()
	usn := udn + "::urn:schemas-upnp-org:device:MediaServer:1"

	msg := "NOTIFY * HTTP/1.1\r\n" +
		"HOST: " + ssdpMulticastAddr + "\r\n" +
		"NT: urn:schemas-upnp-org:device:MediaServer:1\r\n" +
		"NTS: ssdp:byebye\r\n" +
		"USN: " + usn + "\r\n" +
		"\r\n"

	a.sendMulticast([]byte(msg))
	a.log.Debug().Msg("SSDP byebye sent")
}

func (a *SSDPAdvertiser) sendSearchResponse(addr *net.UDPAddr, st string) {
	location := a.location()
	udn := a.server.UDN()
	usn := udn + "::urn:schemas-upnp-org:device:MediaServer:1"

	msg := "HTTP/1.1 200 OK\r\n" +
		"CACHE-CONTROL: max-age=" + fmt.Sprintf("%d", ssdpMaxAge) + "\r\n" +
		"LOCATION: " + location + "\r\n" +
		"ST: " + st + "\r\n" +
		"SERVER: MediaHub/1.0 UPnP/1.0 DLNA/1.50\r\n" +
		"USN: " + usn + "\r\n" +
		"EXT:\r\n" +
		"\r\n"

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		a.log.Debug().Err(err).Msg("failed to dial for M-SEARCH response")
		return
	}
	defer conn.Close()
	conn.Write([]byte(msg))
	a.log.Debug().Str("remote", addr.String()).Str("st", st).Msg("SSDP search response sent")
}

func (a *SSDPAdvertiser) sendMulticast(msg []byte) {
	addr, err := net.ResolveUDPAddr("udp4", ssdpMulticastAddr)
	if err != nil {
		a.log.Debug().Err(err).Msg("failed to resolve multicast addr")
		return
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		a.log.Debug().Err(err).Msg("failed to dial multicast")
		return
	}
	defer conn.Close()
	conn.Write(msg)
}

func (a *SSDPAdvertiser) location() string {
	host := extractHost(a.baseURL)
	return fmt.Sprintf("http://%s:%d/dlna/device.xml", host, a.port)
}

func extractHost(baseURL string) string {
	s := baseURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "localhost"
	}
	return s
}

func extractHeader(msg, header string) string {
	lines := strings.Split(msg, "\r\n")
	prefix := strings.ToUpper(header) + ":"
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), prefix) {
			return strings.TrimSpace(line[strings.Index(strings.ToUpper(line), prefix)+len(prefix):])
		}
	}
	return ""
}
