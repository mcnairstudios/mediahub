package jellyfin

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/favorite"
	"github.com/mcnairstudios/mediahub/pkg/logocache"
	"github.com/mcnairstudios/mediahub/pkg/store"
	realtmdb "github.com/mcnairstudios/mediahub/pkg/tmdb"
)

const (
	viewMoviesID = "f0000000-0000-0000-0000-000000000001"
	viewTVID     = "f0000000-0000-0000-0000-000000000002"
)

type Server struct {
	serverID   string
	serverName string
	auth       auth.Service
	channels   channel.Store
	groups     channel.GroupStore
	streams    store.StreamStore
	programs   epg.ProgramStore
	favorites  favorite.Store
	tmdbCache  *tmdbcache.Cache
	tmdbClient *realtmdb.Client
	imageCache *realtmdb.ImageCache
	logoCache  *logocache.Cache
	log        zerolog.Logger
	tokens     sync.Map
	state      *persistedState
}

type ServerDeps struct {
	ServerName string
	StateDir   string
	Auth       auth.Service
	Channels   channel.Store
	Groups     channel.GroupStore
	Streams    store.StreamStore
	Programs   epg.ProgramStore
	Favorites  favorite.Store
	TMDBCache  *tmdbcache.Cache
	TMDBClient *realtmdb.Client
	ImageCache *realtmdb.ImageCache
	LogoCache  *logocache.Cache
	Log        zerolog.Logger
}

func NewServer(deps ServerDeps) *Server {
	state := loadState(deps.StateDir)
	s := &Server{
		serverID:   generateGUID(),
		serverName: deps.ServerName,
		auth:       deps.Auth,
		channels:   deps.Channels,
		groups:     deps.Groups,
		streams:    deps.Streams,
		programs:   deps.Programs,
		favorites:  deps.Favorites,
		tmdbCache:  deps.TMDBCache,
		tmdbClient: deps.TMDBClient,
		imageCache: deps.ImageCache,
		logoCache:  deps.LogoCache,
		log:        deps.Log.With().Str("component", "jellyfin").Logger(),
		state:      state,
	}
	state.syncTokens(&s.tokens)
	return s
}

func generateGUID() string {
	id := make([]byte, 16)
	rand.Read(id)
	return hex.EncodeToString(id)
}

func jellyfinID(uuid string) string {
	return strings.ReplaceAll(uuid, "-", "")
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	s.registerWebRoutes(mux)
	s.registerPublicRoutes(mux)
	s.registerMediaRoutes(mux)
	s.registerAuthenticatedRoutes(mux)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == "GET" || r.Method == "HEAD") && strings.Contains(r.URL.Path, "/Videos/") && strings.Contains(r.URL.Path, "/stream.") {
			parts := strings.Split(r.URL.Path, "/")
			for i, p := range parts {
				if p == "Videos" && i+2 < len(parts) && strings.HasPrefix(parts[i+2], "stream.") {
					r.SetPathValue("itemId", parts[i+1])
					s.videoStream(w, r)
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) registerWebRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("Jellyfin Server"))
	})
	mux.HandleFunc("GET /web", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web/index.html", http.StatusFound)
	})
	mux.HandleFunc("GET /web/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web/index.html", http.StatusFound)
	})
	mux.HandleFunc("GET /web/index.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<!DOCTYPE html><html><body><h1>MediaHub Jellyfin API</h1><p>Use a Jellyfin client app to connect.</p></body></html>"))
	})
	mux.HandleFunc("GET /web/{file}", s.webFile)
	mux.HandleFunc("GET /favicon.ico", notFound)
}

func (s *Server) registerPublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /System/Info/Public", s.systemInfoPublic)
	mux.HandleFunc("GET /System/Info", s.systemInfo)
	mux.HandleFunc("GET /System/Info/Storage", s.systemInfoStorage)
	mux.HandleFunc("GET /System/Ping", s.ping)
	mux.HandleFunc("POST /System/Ping", s.ping)
	mux.HandleFunc("GET /System/Endpoint", s.systemEndpoint)
	mux.HandleFunc("GET /Branding/Configuration", s.brandingConfig)
	mux.HandleFunc("GET /Branding/Css", s.brandingCSS)
	mux.HandleFunc("GET /Branding/Splashscreen", notFound)
	mux.HandleFunc("GET /QuickConnect/Enabled", s.quickConnectEnabled)
	mux.HandleFunc("POST /QuickConnect/Initiate", s.quickConnectInitiate)
	mux.HandleFunc("GET /Users/Public", s.usersPublic)
	mux.HandleFunc("POST /Users/AuthenticateByName", s.authenticateByName)
	mux.HandleFunc("GET /UserImage", notFound)
	mux.HandleFunc("HEAD /UserImage", notFound)
	mux.HandleFunc("GET /socket", s.websocketStub)
	mux.HandleFunc("POST /ClientLog/Document", s.clientLogDocument)
	mux.HandleFunc("GET /web/ConfigurationPages", s.configurationPages)
	mux.HandleFunc("GET /ScheduledTasks", s.scheduledTasks)
}

func (s *Server) registerMediaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /Items/{itemId}/Images/{imageType}", s.serveImage)
	mux.HandleFunc("GET /Items/{itemId}/Images/{imageType}/{imageIndex}", s.serveImage)
	mux.HandleFunc("HEAD /Items/{itemId}/Images/{imageType}", s.serveImage)
	mux.HandleFunc("HEAD /Items/{itemId}/Images/{imageType}/{imageIndex}", s.serveImage)
	mux.HandleFunc("GET /Persons/{personId}/Images/{imageType}", s.servePersonImage)
	mux.HandleFunc("GET /Videos/{itemId}/stream", s.videoStream)
	mux.HandleFunc("HEAD /Videos/{itemId}/stream", s.videoStream)
	mux.HandleFunc("GET /Videos/{itemId}/master.m3u8", s.hlsMasterPlaylist)
	mux.HandleFunc("GET /Videos/{itemId}/main.m3u8", s.hlsMediaPlaylist)
	mux.HandleFunc("GET /Videos/{itemId}/live.m3u8", s.hlsLivePlaylist)
	mux.HandleFunc("GET /Videos/{itemId}/hls1/{playlistId}/{segment}", s.hlsSegment)
	mux.HandleFunc("GET /Playback/BitrateTest", s.bitrateTest)
}

func (s *Server) registerAuthenticatedRoutes(mux *http.ServeMux) {
	mux.Handle("GET /Users/Me", s.requireAuth(http.HandlerFunc(s.usersMe)))
	mux.Handle("GET /Users", s.requireAuth(http.HandlerFunc(s.usersList)))
	mux.Handle("GET /Users/{userId}", s.requireAuth(http.HandlerFunc(s.userByID)))
	mux.Handle("POST /Users/Configuration", s.requireAuth(http.HandlerFunc(noContent)))
	mux.Handle("POST /Users/Password", s.requireAuth(http.HandlerFunc(noContent)))

	mux.Handle("GET /UserViews", s.requireAuth(http.HandlerFunc(s.userViews)))
	mux.Handle("GET /Items", s.requireAuth(http.HandlerFunc(s.listItems)))
	mux.Handle("GET /Items/Filters", s.requireAuth(http.HandlerFunc(s.listFilters)))
	mux.Handle("GET /Items/{itemId}", s.requireAuth(http.HandlerFunc(s.itemDetail)))
	mux.Handle("GET /Items/Latest", s.requireAuth(http.HandlerFunc(s.latestItems)))
	mux.Handle("GET /Items/Resume", s.requireAuth(http.HandlerFunc(s.listResumeItems)))
	mux.Handle("GET /Items/Counts", s.requireAuth(http.HandlerFunc(s.itemCounts)))
	mux.Handle("GET /Items/Suggestions", s.requireAuth(http.HandlerFunc(s.listSuggestions)))
	mux.Handle("GET /UserItems/Resume", s.requireAuth(http.HandlerFunc(s.listResumeItems)))
	mux.Handle("GET /Users/{userId}/Items", s.requireAuth(http.HandlerFunc(s.listItems)))
	mux.Handle("GET /Users/{userId}/Items/Latest", s.requireAuth(http.HandlerFunc(s.latestItems)))
	mux.Handle("GET /Users/{userId}/Items/Resume", s.requireAuth(http.HandlerFunc(s.listResumeItems)))
	mux.Handle("GET /Users/{userId}/Items/{itemId}", s.requireAuth(http.HandlerFunc(s.itemDetail)))
	mux.Handle("GET /Shows/NextUp", s.requireAuth(http.HandlerFunc(s.listResumeItems)))
	mux.Handle("GET /Shows/{seriesId}/Seasons", s.requireAuth(http.HandlerFunc(s.listSeasons)))
	mux.Handle("GET /Shows/{seriesId}/Episodes", s.requireAuth(http.HandlerFunc(s.listEpisodes)))
	mux.Handle("GET /Items/{itemId}/Similar", s.requireAuth(http.HandlerFunc(s.listSimilarItems)))
	mux.Handle("GET /Items/{itemId}/LocalTrailers", s.requireAuth(http.HandlerFunc(s.listSpecialFeatures)))
	mux.Handle("GET /Items/{itemId}/SpecialFeatures", s.requireAuth(http.HandlerFunc(s.listSpecialFeatures)))
	mux.Handle("GET /Items/{itemId}/ThemeMedia", s.requireAuth(http.HandlerFunc(s.listSpecialFeatures)))
	mux.Handle("GET /Items/{itemId}/ThemeSongs", s.requireAuth(http.HandlerFunc(s.listSpecialFeatures)))
	mux.Handle("GET /Items/{itemId}/ThemeVideos", s.requireAuth(http.HandlerFunc(s.listSpecialFeatures)))
	mux.Handle("GET /Items/{itemId}/InstantMix", s.requireAuth(http.HandlerFunc(s.listSpecialFeatures)))
	mux.Handle("GET /Items/{itemId}/Intros", s.requireAuth(http.HandlerFunc(s.emptyQueryResult)))

	mux.Handle("POST /Items/{itemId}/PlaybackInfo", s.requireAuth(http.HandlerFunc(s.playbackInfo)))

	mux.Handle("GET /Persons", s.requireAuth(http.HandlerFunc(s.emptyQueryResult)))
	mux.Handle("GET /Persons/{personId}", s.requireAuth(http.HandlerFunc(s.personDetail)))
	mux.Handle("GET /Studios", s.requireAuth(http.HandlerFunc(s.emptyQueryResult)))
	mux.Handle("GET /Artists", s.requireAuth(http.HandlerFunc(s.emptyQueryResult)))
	mux.Handle("GET /Genres", s.requireAuth(http.HandlerFunc(s.emptyQueryResult)))
	mux.Handle("GET /Genres/{genreName}", s.requireAuth(http.HandlerFunc(s.genreDetail)))

	mux.Handle("POST /UserPlayedItems/{itemId}", s.requireAuth(http.HandlerFunc(s.markPlayed)))
	mux.Handle("DELETE /UserPlayedItems/{itemId}", s.requireAuth(http.HandlerFunc(s.markPlayed)))
	mux.Handle("POST /UserFavoriteItems/{itemId}", s.requireAuth(http.HandlerFunc(s.markFavorite)))
	mux.Handle("DELETE /UserFavoriteItems/{itemId}", s.requireAuth(http.HandlerFunc(s.markFavorite)))
	mux.Handle("GET /UserItems/{itemId}/UserData", s.requireAuth(http.HandlerFunc(s.getUserData)))
	mux.Handle("POST /UserItems/{itemId}/UserData", s.requireAuth(http.HandlerFunc(noContent)))
	mux.Handle("POST /UserItems/{itemId}/Rating", s.requireAuth(http.HandlerFunc(s.getUserData)))
	mux.Handle("DELETE /UserItems/{itemId}/Rating", s.requireAuth(http.HandlerFunc(s.getUserData)))

	mux.Handle("GET /LiveTv/Info", s.requireAuth(http.HandlerFunc(s.liveTvInfo)))
	mux.Handle("GET /LiveTv/Channels", s.requireAuth(http.HandlerFunc(s.liveTvChannels)))
	mux.Handle("GET /LiveTv/Programs", s.requireAuth(http.HandlerFunc(s.liveTvPrograms)))
	mux.Handle("GET /LiveTv/Programs/Recommended", s.requireAuth(http.HandlerFunc(s.liveTvPrograms)))
	mux.Handle("POST /LiveTv/Programs", s.requireAuth(http.HandlerFunc(s.liveTvPrograms)))
	mux.Handle("GET /LiveTv/GuideInfo", s.requireAuth(http.HandlerFunc(s.liveTvGuideInfo)))

	mux.Handle("GET /Sessions", s.requireAuth(http.HandlerFunc(s.sessionsGet)))
	mux.Handle("GET /System/ActivityLog/Entries", s.requireAuth(http.HandlerFunc(s.emptyQueryResult)))
	mux.Handle("GET /Notifications/{userId}/Summary", s.requireAuth(http.HandlerFunc(s.notificationSummary)))
	mux.Handle("POST /Sessions/Capabilities", s.requireAuth(http.HandlerFunc(noContent)))
	mux.Handle("POST /Sessions/Capabilities/Full", s.requireAuth(http.HandlerFunc(noContent)))
	mux.Handle("POST /Sessions/Playing", s.requireAuth(http.HandlerFunc(noContent)))
	mux.Handle("POST /Sessions/Playing/Progress", s.requireAuth(http.HandlerFunc(noContent)))
	mux.Handle("POST /Sessions/Playing/Stopped", s.requireAuth(http.HandlerFunc(noContent)))
	mux.Handle("GET /DisplayPreferences/{id}", s.requireAuth(http.HandlerFunc(s.displayPreferences)))
	mux.Handle("POST /DisplayPreferences/{id}", s.requireAuth(http.HandlerFunc(noContent)))
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

func noContent(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) respondJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	w.Write(data)
}

func (s *Server) emptyQueryResult(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, emptyResult())
}

func (s *Server) jellyfinBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host
}

func (s *Server) systemInfoPublic(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, PublicSystemInfo{
		LocalAddress:           s.jellyfinBaseURL(r),
		ServerName:             s.serverName,
		Version:                "10.10.6",
		ProductName:            "Jellyfin Server",
		OperatingSystem:        "Linux",
		ID:                     s.serverID,
		StartupWizardCompleted: true,
	})
}

func (s *Server) systemInfo(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, SystemInfo{
		PublicSystemInfo: PublicSystemInfo{
			LocalAddress:           s.jellyfinBaseURL(r),
			ServerName:             s.serverName,
			Version:                "10.10.6",
			ProductName:            "Jellyfin Server",
			OperatingSystem:        "Linux",
			ID:                     s.serverID,
			StartupWizardCompleted: true,
		},
		OperatingSystemDisplayName: "Linux",
		WebSocketPortNumber:        8096,
		SupportsLibraryMonitor:     true,
		CanSelfRestart:             true,
	})
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Jellyfin Server"))
}

func (s *Server) brandingConfig(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, BrandingConfiguration{})
}

func (s *Server) brandingCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
}

func (s *Server) quickConnectEnabled(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, false)
}

func (s *Server) systemEndpoint(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{
		"IsLocal":     true,
		"IsInNetwork": true,
	})
}

func (s *Server) scheduledTasks(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, []any{})
}

func (s *Server) quickConnectInitiate(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "QuickConnect not supported", http.StatusBadRequest)
}

func (s *Server) systemInfoStorage(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"Drives": []any{}})
}

func (s *Server) websocketStub(w http.ResponseWriter, r *http.Request) {
	s.log.Info().Str("remote", r.RemoteAddr).Str("upgrade", r.Header.Get("Upgrade")).Msg("websocket connection attempt")
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "websocket required", http.StatusBadRequest)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket not supported", http.StatusInternalServerError)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	key := r.Header.Get("Sec-WebSocket-Key")
	accept := computeWebSocketAccept(key)
	buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	buf.WriteString("Upgrade: websocket\r\n")
	buf.WriteString("Connection: Upgrade\r\n")
	buf.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	buf.Flush()

	b := make([]byte, 1)
	for {
		if _, err := conn.Read(b); err != nil {
			return
		}
	}
}

func computeWebSocketAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (s *Server) clientLogDocument(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.log.Debug().Str("body", string(body)).Msg("client log document")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) configurationPages(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, []any{})
}

func (s *Server) webFile(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	switch file {
	case "config.json":
		s.respondJSON(w, http.StatusOK, map[string]any{
			"menuLinks": []any{}, "multiserver": false, "themes": []any{}, "plugins": []any{},
		})
	case "manifest.json":
		s.respondJSON(w, http.StatusOK, map[string]any{
			"name": "MediaHub", "short_name": "MediaHub", "start_url": "/web/index.html",
			"display": "standalone", "background_color": "#1a1d23", "theme_color": "#3b82f6",
		})
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) personDetail(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("personId")
	s.respondJSON(w, http.StatusOK, BaseItemDto{
		Name: personID, ServerID: s.serverID,
		ID: personID, Type: "Person",
	})
}

func (s *Server) genreDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("genreName")
	s.respondJSON(w, http.StatusOK, BaseItemDto{
		Name: name, ServerID: s.serverID,
		ID: fmt.Sprintf("genre_%x", hashString(name)), Type: "Genre",
	})
}

func (s *Server) notificationSummary(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]int{"UnreadCount": 0, "MaxUnreadCount": 0})
}

func (s *Server) getUserData(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, UserItemData{Key: r.PathValue("itemId")})
}

func (s *Server) markPlayed(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, UserItemData{
		Played: r.Method == "POST",
		Key:    r.PathValue("itemId"),
	})
}

func (s *Server) markFavorite(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	userID := s.authenticatedUserID(r)
	isFav := r.Method == "POST"

	if s.favorites != nil && userID != "" {
		if isFav {
			s.favorites.Add(r.Context(), userID, addDashes(itemID))
		} else {
			s.favorites.Remove(r.Context(), userID, addDashes(itemID))
		}
	}

	s.respondJSON(w, http.StatusOK, UserItemData{
		IsFavorite: isFav,
		Key:        itemID,
	})
}

func (s *Server) bitrateTest(w http.ResponseWriter, r *http.Request) {
	size := 1000000
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if n, err := strconv.Atoi(sizeStr); err == nil && n > 0 && n <= 10000000 {
			size = n
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(size))
	buf := make([]byte, 65536)
	written := 0
	for written < size {
		chunk := size - written
		if chunk > len(buf) {
			chunk = len(buf)
		}
		w.Write(buf[:chunk])
		written += chunk
	}
}

func (s *Server) listSpecialFeatures(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, []BaseItemDto{})
}

func (s *Server) usersPublic(w http.ResponseWriter, r *http.Request) {
	users, _ := s.auth.ListUsers(r.Context())
	var result []UserDto
	for _, u := range users {
		result = append(result, UserDto{
			Name:                  u.Username,
			ServerID:              s.serverID,
			ID:                    jellyfinID(u.ID),
			HasPassword:           true,
			HasConfiguredPassword: true,
			Policy:                defaultPolicy(u.IsAdmin),
			Configuration:         defaultUserConfig(),
		})
	}
	if result == nil {
		result = []UserDto{}
	}
	s.respondJSON(w, http.StatusOK, result)
}

func (s *Server) usersMe(w http.ResponseWriter, r *http.Request) {
	userID := s.authenticatedUserID(r)
	user := s.lookupUser(r, userID)
	s.respondJSON(w, http.StatusOK, user)
}

func (s *Server) usersList(w http.ResponseWriter, r *http.Request) {
	userID := s.authenticatedUserID(r)
	user := s.lookupUser(r, userID)
	s.respondJSON(w, http.StatusOK, []UserDto{user})
}

func (s *Server) userByID(w http.ResponseWriter, r *http.Request) {
	s.usersMe(w, r)
}

func (s *Server) lookupUser(r *http.Request, userID string) UserDto {
	users, _ := s.auth.ListUsers(r.Context())
	name := "user"
	isAdmin := false
	for _, u := range users {
		if u.ID == userID {
			name = u.Username
			isAdmin = u.IsAdmin
			break
		}
	}
	now := time.Now().UTC()
	return UserDto{
		Name:                  name,
		ServerID:              s.serverID,
		ServerName:            s.serverName,
		ID:                    jellyfinID(userID),
		HasPassword:           true,
		HasConfiguredPassword: true,
		LastLoginDate:         &now,
		LastActivityDate:      &now,
		Configuration:         defaultUserConfig(),
		Policy:                defaultPolicy(isAdmin),
	}
}

func (s *Server) itemCounts(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]int{
		"MovieCount": 0, "SeriesCount": 0, "EpisodeCount": 0,
		"ArtistCount": 0, "ProgramCount": 0, "TrailerCount": 0,
		"SongCount": 0, "AlbumCount": 0, "MusicVideoCount": 0,
		"BoxSetCount": 0, "BookCount": 0, "ItemCount": 0,
	})
}

func (s *Server) sessionsGet(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, []SessionInfo{})
}

func (s *Server) displayPreferences(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	client := r.URL.Query().Get("client")
	if client == "" {
		client = s.extractAuthField(r, "Client")
	}
	s.respondJSON(w, http.StatusOK, map[string]any{
		"Id":                  id,
		"SortBy":              "SortName",
		"SortOrder":           "Ascending",
		"RememberIndexing":    false,
		"RememberSorting":     false,
		"Client":              client,
		"PrimaryImageHeight":  250,
		"PrimaryImageWidth":   166,
		"ScrollDirection":     "Horizontal",
		"ShowBackdrop":        true,
		"ShowSidebar":         false,
		"CustomPrefs": map[string]string{
			"chromecastVersion":          "stable",
			"skipForwardLength":          "30000",
			"skipBackLength":             "10000",
			"enableNextVideoInfoOverlay": "true",
			"tvhome":                     "",
		},
	})
}

func (s *Server) rootFolderID() string {
	h := hashString(s.serverID)
	id := fmt.Sprintf("e9d5%04x%08x%016x", h>>16, h, uint64(h)*6364136223846793005)
	if len(id) > 32 {
		id = id[:32]
	}
	return id
}

func (s *Server) newCollectionFolderItem(name, id, collectionType string, imgTags map[string]string) BaseItemDto {
	dashlessID := stripDashes(id)
	dashedID := addDashes(dashlessID)
	return BaseItemDto{
		Name:                     name,
		ServerID:                 s.serverID,
		ID:                       dashlessID,
		DateCreated:              "2026-01-01T00:00:00.0000000Z",
		DateLastMediaAdded:       "0001-01-01T00:00:00.0000000Z",
		CanDelete:                boolPtr(false),
		CanDownload:              boolPtr(false),
		SortName:                 strings.ToLower(name),
		ExternalUrls:             []any{},
		EnableMediaSourceDisplay: true,
		Taglines:                 []string{},
		Genres:                   []string{},
		PlayAccess:               "Full",
		RemoteTrailers:           []any{},
		ProviderIds:              map[string]string{},
		IsFolder:                 true,
		ParentID:                 s.rootFolderID(),
		Type:                     "CollectionFolder",
		CollectionType:           collectionType,
		People:                   []PersonDto{},
		Studios:                  []NameIDPair{},
		GenreItems:               []NameIDPair{},
		UserData: &UserItemData{
			Key:    dashedID,
			ItemID: dashlessID,
		},
		SpecialFeatureCount:  0,
		DisplayPreferencesId: dashlessID,
		Tags:                 []string{},
		ImageTags:            imgTags,
		BackdropImageTags:    []string{},
		ImageBlurHashes:      map[string]any{},
		LocationType:         "FileSystem",
		MediaType:            "Unknown",
		LockedFields:         []string{},
		LockData:             boolPtr(false),
	}
}

func (s *Server) userViews(w http.ResponseWriter, r *http.Request) {
	views := []BaseItemDto{
		s.newCollectionFolderItem("Movies", viewMoviesID, "movies", map[string]string{}),
		s.newCollectionFolderItem("TV Shows", viewTVID, "tvshows", map[string]string{}),
	}

	s.respondJSON(w, http.StatusOK, BaseItemDtoQueryResult{
		Items: views, TotalRecordCount: len(views),
	})
}

func (s *Server) newChannelItem(ch *channel.Channel) BaseItemDto {
	chID := stripDashes(ch.ID)
	item := BaseItemDto{
		Name: ch.Name, ServerID: s.serverID,
		ID: chID, Type: "Video",
		MediaType: "Video", IsFolder: false,
		ImageTags: map[string]string{},
		UserData:  &UserItemData{Key: ch.ID},
		MediaSources: []MediaSource{{
			Protocol: "Http", ID: chID, Type: "Default",
			Name: ch.Name, IsRemote: true, IsInfiniteStream: true,
			SupportsTranscoding: true, SupportsDirectStream: true,
			TranscodingURL: channelStreamURL(chID),
		}},
	}
	if ch.LogoURL != "" {
		item.ImageTags["Primary"] = "logo"
	}
	return item
}

func (s *Server) newLiveTvChannelItem(ch *channel.Channel, index int) BaseItemDto {
	chID := stripDashes(ch.ID)
	item := BaseItemDto{
		Name: ch.Name, ServerID: s.serverID,
		ID: chID, Type: "LiveTvChannel",
		MediaType: "Video", IsFolder: false,
		ChannelNumber: fmt.Sprintf("%d", index+1),
		ImageTags:     map[string]string{},
		UserData:      &UserItemData{Key: ch.ID},
		MediaSources: []MediaSource{{
			Protocol: "Http", ID: chID, Type: "Default",
			Name: ch.Name, IsRemote: true, IsInfiniteStream: true,
			SupportsTranscoding: true, SupportsDirectStream: true,
			TranscodingURL: channelStreamURL(chID),
		}},
	}
	if ch.LogoURL != "" {
		item.ImageTags["Primary"] = "logo"
		item.ChannelPrimaryImageTag = "logo"
	}
	return item
}
