package hdhr

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/rs/zerolog"
)

type runningDevice struct {
	port   int
	server *http.Server
	cancel context.CancelFunc
	ssdp   *SSDPAdvertiser
}

type DeviceManager struct {
	deviceStore  DeviceStore
	channelStore channel.Store
	baseURL      string
	log          zerolog.Logger
	mu           sync.Mutex
	running      map[string]*runningDevice
}

func NewDeviceManager(deviceStore DeviceStore, channelStore channel.Store, baseURL string, log zerolog.Logger) *DeviceManager {
	return &DeviceManager{
		deviceStore:  deviceStore,
		channelStore: channelStore,
		baseURL:      baseURL,
		log:          log.With().Str("component", "hdhr_device_manager").Logger(),
		running:      make(map[string]*runningDevice),
	}
}

func (m *DeviceManager) Run(ctx context.Context) {
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return
	}

	m.sync(ctx)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.stopAll()
			return
		case <-ticker.C:
			m.sync(ctx)
		}
	}
}

func (m *DeviceManager) sync(ctx context.Context) {
	devices, err := m.deviceStore.List(ctx)
	if err != nil {
		m.log.Error().Err(err).Msg("failed to list HDHR devices")
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	desired := make(map[string]int)
	for _, d := range devices {
		if d.IsEnabled && d.Port > 0 {
			desired[d.ID] = d.Port
		}
	}

	for id, rd := range m.running {
		wantPort, ok := desired[id]
		if !ok || wantPort != rd.port {
			m.log.Info().Str("device_id", id).Int("port", rd.port).Msg("stopping HDHR device server")
			rd.cancel()
			rd.server.Close()
			delete(m.running, id)
		}
	}

	for _, d := range devices {
		if !d.IsEnabled || d.Port <= 0 {
			continue
		}
		if _, exists := m.running[d.ID]; exists {
			continue
		}
		m.startDevice(ctx, d)
	}
}

func (m *DeviceManager) startDevice(parentCtx context.Context, device Device) {
	host := m.extractHost()
	baseURL := fmt.Sprintf("http://%s:%d", host, device.Port)

	srv := NewDeviceServer(m.channelStore, &device, baseURL)

	devCtx, cancel := context.WithCancel(parentCtx)
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", device.Port),
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
		BaseContext:  func(_ net.Listener) context.Context { return devCtx },
	}

	ssdp := NewSSDPAdvertiser(baseURL, 30*time.Second, m.log)
	ssdp.SetDeviceID(device.DeviceUUID)
	ssdp.SetFriendlyName(device.Name)

	m.running[device.ID] = &runningDevice{
		port:   device.Port,
		server: httpSrv,
		cancel: cancel,
		ssdp:   ssdp,
	}

	m.log.Info().
		Str("device", device.Name).
		Str("uuid", device.DeviceUUID).
		Int("port", device.Port).
		Str("base_url", baseURL).
		Msg("starting HDHR device server")

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.log.Error().Err(err).Int("port", device.Port).Msg("HDHR device server error")
		}
	}()

	go ssdp.Run(devCtx)
}

func (m *DeviceManager) extractHost() string {
	u, err := url.Parse(m.baseURL)
	if err != nil {
		return "localhost"
	}
	host := u.Hostname()
	if host == "" {
		return "localhost"
	}
	return host
}

func (m *DeviceManager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, rd := range m.running {
		m.log.Info().Str("device_id", id).Int("port", rd.port).Msg("stopping HDHR device server")
		rd.cancel()
		rd.server.Close()
		delete(m.running, id)
	}
}
