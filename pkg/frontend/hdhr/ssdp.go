package hdhr

import (
	"context"
	"fmt"
	"net"
	"net/url"
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
	baseURL          string
	log              zerolog.Logger
	announceInterval time.Duration
	mu               sync.Mutex
	deviceID         string
	friendlyName     string
}

func NewSSDPAdvertiser(baseURL string, announceInterval time.Duration, log zerolog.Logger) *SSDPAdvertiser {
	if announceInterval <= 0 {
		announceInterval = 30 * time.Second
	}
	return &SSDPAdvertiser{
		baseURL:          baseURL,
		log:              log.With().Str("component", "hdhr-ssdp").Logger(),
		announceInterval: announceInterval,
	}
}

func (a *SSDPAdvertiser) SetDeviceID(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deviceID = id
}

func (a *SSDPAdvertiser) SetFriendlyName(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.friendlyName = name
}

func (a *SSDPAdvertiser) getDeviceID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.deviceID != "" {
		return a.deviceID
	}
	return DefaultDeviceID
}

func (a *SSDPAdvertiser) Run(ctx context.Context) {
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return
	}

	listener, err := a.startListener()
	if err != nil {
		a.log.Error().Err(err).Msg("failed to start HDHR SSDP listener")
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

		st := extractSSDPHeader(msg, "ST")
		if st != "ssdp:all" && st != "upnp:rootdevice" && st != "urn:schemas-upnp-org:device:MediaServer:1" {
			continue
		}

		a.sendSearchResponse(remoteAddr, st)
	}
}

func (a *SSDPAdvertiser) sendAlive() {
	location := a.location()
	usn := "uuid:" + a.getDeviceID() + "::upnp:rootdevice"

	msg := "NOTIFY * HTTP/1.1\r\n" +
		"HOST: " + ssdpMulticastAddr + "\r\n" +
		"CACHE-CONTROL: max-age=" + fmt.Sprintf("%d", ssdpMaxAge) + "\r\n" +
		"LOCATION: " + location + "\r\n" +
		"NT: upnp:rootdevice\r\n" +
		"NTS: ssdp:alive\r\n" +
		"SERVER: HDHomeRun/1.0 UPnP/1.0\r\n" +
		"USN: " + usn + "\r\n" +
		"\r\n"

	a.sendMulticast([]byte(msg))
	a.log.Debug().Str("location", location).Msg("HDHR SSDP alive sent")
}

func (a *SSDPAdvertiser) sendByeBye() {
	usn := "uuid:" + a.getDeviceID() + "::upnp:rootdevice"

	msg := "NOTIFY * HTTP/1.1\r\n" +
		"HOST: " + ssdpMulticastAddr + "\r\n" +
		"NT: upnp:rootdevice\r\n" +
		"NTS: ssdp:byebye\r\n" +
		"USN: " + usn + "\r\n" +
		"\r\n"

	a.sendMulticast([]byte(msg))
	a.log.Debug().Msg("HDHR SSDP byebye sent")
}

func (a *SSDPAdvertiser) sendSearchResponse(addr *net.UDPAddr, st string) {
	location := a.location()
	usn := "uuid:" + a.getDeviceID() + "::upnp:rootdevice"

	msg := "HTTP/1.1 200 OK\r\n" +
		"CACHE-CONTROL: max-age=" + fmt.Sprintf("%d", ssdpMaxAge) + "\r\n" +
		"LOCATION: " + location + "\r\n" +
		"ST: " + st + "\r\n" +
		"SERVER: HDHomeRun/1.0 UPnP/1.0\r\n" +
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
	a.log.Debug().Str("remote", addr.String()).Str("st", st).Msg("HDHR SSDP search response sent")
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
	host := ssdpExtractHost(a.baseURL)
	return fmt.Sprintf("http://%s/device.xml", host)
}

func ssdpExtractHost(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "localhost"
	}
	host := u.Host
	if host == "" {
		return "localhost"
	}
	return host
}

func extractSSDPHeader(msg, header string) string {
	lines := strings.Split(msg, "\r\n")
	prefix := strings.ToUpper(header) + ":"
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), prefix) {
			return strings.TrimSpace(line[strings.Index(strings.ToUpper(line), prefix)+len(prefix):])
		}
	}
	return ""
}
