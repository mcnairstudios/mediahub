package jellyfin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/store"
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
	tmdbCache  *tmdb.Cache
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
	TMDBCache  *tmdb.Cache
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
		tmdbCache:  deps.TMDBCache,
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

	s.registerPublicRoutes(mux)
	s.registerMediaRoutes(mux)
	s.registerAuthenticatedRoutes(mux)

	return mux
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) registerPublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /System/Info/Public", s.systemInfoPublic)
	mux.HandleFunc("GET /System/Info", s.systemInfo)
	mux.HandleFunc("GET /System/Ping", s.ping)
	mux.HandleFunc("POST /System/Ping", s.ping)
	mux.HandleFunc("GET /System/Endpoint", s.systemEndpoint)
	mux.HandleFunc("GET /Branding/Configuration", s.brandingConfig)
	mux.HandleFunc("GET /Branding/Css", s.brandingCSS)
	mux.HandleFunc("GET /Branding/Splashscreen", notFound)
	mux.HandleFunc("GET /QuickConnect/Enabled", s.quickConnectEnabled)
	mux.HandleFunc("GET /Users/Public", s.usersPublic)
	mux.HandleFunc("POST /Users/AuthenticateByName", s.authenticateByName)
	mux.HandleFunc("GET /ScheduledTasks", s.scheduledTasks)
}

func (s *Server) registerMediaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /Items/{itemId}/Images/{imageType}", s.serveImage)
	mux.HandleFunc("GET /Items/{itemId}/Images/{imageType}/{imageIndex}", s.serveImage)
	mux.HandleFunc("HEAD /Items/{itemId}/Images/{imageType}", s.serveImage)
	mux.HandleFunc("HEAD /Items/{itemId}/Images/{imageType}/{imageIndex}", s.serveImage)
}

func (s *Server) registerAuthenticatedRoutes(mux *http.ServeMux) {
	mux.Handle("GET /Users/Me", s.requireAuth(http.HandlerFunc(s.usersMe)))
	mux.Handle("GET /Users", s.requireAuth(http.HandlerFunc(s.usersList)))
	mux.Handle("GET /Users/{userId}", s.requireAuth(http.HandlerFunc(s.userByID)))

	mux.Handle("GET /UserViews", s.requireAuth(http.HandlerFunc(s.userViews)))
	mux.Handle("GET /Items", s.requireAuth(http.HandlerFunc(s.listItems)))
	mux.Handle("GET /Items/{itemId}", s.requireAuth(http.HandlerFunc(s.itemDetail)))
	mux.Handle("GET /Items/Latest", s.requireAuth(http.HandlerFunc(s.latestItems)))
	mux.Handle("GET /Items/Resume", s.requireAuth(http.HandlerFunc(s.listResumeItems)))
	mux.Handle("GET /Items/Counts", s.requireAuth(http.HandlerFunc(s.itemCounts)))
	mux.Handle("GET /Users/{userId}/Items", s.requireAuth(http.HandlerFunc(s.listItems)))
	mux.Handle("GET /Users/{userId}/Items/Latest", s.requireAuth(http.HandlerFunc(s.latestItems)))
	mux.Handle("GET /Users/{userId}/Items/Resume", s.requireAuth(http.HandlerFunc(s.listResumeItems)))
	mux.Handle("GET /Users/{userId}/Items/{itemId}", s.requireAuth(http.HandlerFunc(s.itemDetail)))
	mux.Handle("GET /Shows/NextUp", s.requireAuth(http.HandlerFunc(s.listResumeItems)))

	mux.Handle("POST /Items/{itemId}/PlaybackInfo", s.requireAuth(http.HandlerFunc(s.playbackInfo)))

	mux.Handle("GET /LiveTv/Channels", s.requireAuth(http.HandlerFunc(s.liveTvChannels)))
	mux.Handle("GET /LiveTv/Info", s.requireAuth(http.HandlerFunc(s.liveTvInfo)))

	mux.Handle("GET /Sessions", s.requireAuth(http.HandlerFunc(s.sessionsGet)))
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

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	parentID := firstOf(q, "parentId", "ParentId")
	itemTypes := strings.Join(append(q["includeItemTypes"], q["IncludeItemTypes"]...), ",")
	searchTerm := strings.ToLower(firstOf(q, "searchTerm", "SearchTerm"))
	sortBy := firstOf(q, "sortBy", "SortBy")
	sortOrder := firstOf(q, "sortOrder", "SortOrder")

	ctx := r.Context()

	hasMovies := parentID == viewMoviesID || strings.Contains(itemTypes, "Movie")
	hasSeries := parentID == viewTVID || strings.Contains(itemTypes, "Series")

	var items []BaseItemDto
	if hasMovies || (!hasSeries && searchTerm != "") {
		items = append(items, s.buildStreamItems(ctx, searchTerm)...)
	}

	if !hasMovies && !hasSeries && searchTerm == "" {
		s.respondJSON(w, http.StatusOK, emptyResult())
		return
	}

	sortItems(items, sortBy, sortOrder)
	s.paginateAndRespond(w, r, items)
}

func (s *Server) itemDetail(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	ctx := r.Context()

	if itemID == viewMoviesID || itemID == viewTVID {
		s.userViews(w, r)
		return
	}

	if s.streams != nil {
		if stream, err := s.streams.Get(ctx, addDashes(itemID)); err == nil && stream != nil {
			item := s.streamToItem(stream)
			s.respondJSON(w, http.StatusOK, item)
			return
		}
	}

	if s.channels != nil {
		if ch, err := s.channels.Get(ctx, addDashes(itemID)); err == nil && ch != nil {
			item := s.newChannelItem(ch)
			s.respondJSON(w, http.StatusOK, item)
			return
		}
	}

	s.respondJSON(w, http.StatusOK, BaseItemDto{
		Name: "Unknown", ServerID: s.serverID, ID: itemID, Type: "Video",
	})
}

func (s *Server) latestItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items := s.buildStreamItems(ctx, "")
	sortItems(items, "PremiereDate", "Descending")

	limit := 20
	if l := firstOf(r.URL.Query(), "limit", "Limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if len(items) > limit {
		items = items[:limit]
	}
	if items == nil {
		items = []BaseItemDto{}
	}

	s.respondJSON(w, http.StatusOK, items)
}

func (s *Server) listResumeItems(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, emptyResult())
}

func (s *Server) playbackInfo(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")

	playSessionID := itemID
	if len(playSessionID) > 16 {
		playSessionID = playSessionID[:16]
	}

	ms := MediaSource{
		Protocol: "Http", ID: itemID, Type: "Default", Name: "Default",
		Container: "mp4", IsRemote: true,
		SupportsTranscoding:     true,
		SupportsDirectStream:    false,
		SupportsDirectPlay:      false,
		DefaultAudioStreamIndex: 1,
		TranscodingURL:          fmt.Sprintf("/Videos/%s/master.m3u8?MediaSourceId=%s&PlaySessionId=%s", itemID, itemID, playSessionID),
		TranscodingSubProtocol:  "hls",
		TranscodingContainer:    "ts",
		MediaStreams: []MediaStream{
			{Type: "Video", Codec: "h264", Index: 0, IsDefault: true, Width: 1920, Height: 1080},
			{Type: "Audio", Codec: "aac", Index: 1, IsDefault: true, Channels: 2},
		},
	}

	s.respondJSON(w, http.StatusOK, map[string]any{
		"MediaSources":  []MediaSource{ms},
		"PlaySessionId": playSessionID,
	})
}

func (s *Server) liveTvInfo(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{
		"Services": []any{}, "IsEnabled": true, "EnabledUsers": []string{},
	})
}

func (s *Server) liveTvChannels(w http.ResponseWriter, r *http.Request) {
	if s.channels == nil {
		s.respondJSON(w, http.StatusOK, emptyResult())
		return
	}

	channels, err := s.channels.List(r.Context())
	if err != nil {
		s.respondJSON(w, http.StatusOK, emptyResult())
		return
	}

	var items []BaseItemDto
	for i, ch := range channels {
		item := s.newLiveTvChannelItem(&ch, i)
		items = append(items, item)
	}
	if items == nil {
		items = []BaseItemDto{}
	}

	s.respondJSON(w, http.StatusOK, BaseItemDtoQueryResult{Items: items, TotalRecordCount: len(items)})
}

func (s *Server) serveImage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
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
