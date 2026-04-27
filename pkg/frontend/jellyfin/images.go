package jellyfin

import (
	"net/http"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/tmdb"
)

func (s *Server) serveImage(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	imageType := r.PathValue("imageType")

	isBackdrop := strings.EqualFold(imageType, "Backdrop")

	if strings.HasPrefix(itemID, "person_") {
		s.servePersonImage(w, r)
		return
	}

	if strings.HasPrefix(itemID, "group_") {
		s.serveGroupImage(w, r, addDashes(strings.TrimPrefix(itemID, "group_")))
		return
	}

	if isGroupItemID(itemID) {
		s.serveGroupImage(w, r, groupUUIDFromItemID(itemID))
		return
	}

	if strings.HasPrefix(itemID, "cccc") || isSeasonItemID(itemID) {
		seriesID := itemID
		if isSeasonItemID(itemID) {
			h, _, ok := parseSeasonItemID(itemID)
			if ok {
				seriesID = seriesIDFromName(lookupSeriesNameByHash(s, r, h))
			}
		}
		s.serveSeriesImage(w, r, seriesID, isBackdrop)
		return
	}

	if s.streams != nil {
		if stream, err := s.streams.Get(r.Context(), addDashes(itemID)); err == nil && stream != nil {
			lookupName := stream.Name
			mediaType := stream.VODType

			if stream.VODType == "series" && stream.Season > 0 && stream.Episode > 0 {
				if ep := s.lookupEpisode(lookupName, stream.Season, stream.Episode); ep != nil && ep.StillPath != "" {
					s.serveTMDBImage(w, r, "w300", ep.StillPath)
					return
				}
			}

			if isBackdrop {
				bd := s.lookupBackdrop(lookupName, mediaType)
				if bd != "" {
					s.serveTMDBImage(w, r, "w1280", bd)
					return
				}
			}

			poster := s.lookupPoster(lookupName, mediaType)
			if poster != "" {
				s.serveTMDBImage(w, r, "w500", poster)
				return
			}

			if s.logoCache != nil && stream.TvgLogo != "" {
				resolved := s.logoCache.Resolve(stream.TvgLogo)
				if resolved != "" && !strings.HasPrefix(resolved, "data:") {
					http.Redirect(w, r, resolved, http.StatusTemporaryRedirect)
					return
				}
			}
		}
	}

	if s.channels != nil {
		if ch, err := s.channels.Get(r.Context(), addDashes(itemID)); err == nil && ch != nil && s.logoCache != nil && ch.LogoURL != "" {
			resolved := s.logoCache.Resolve(ch.LogoURL)
			if resolved != "" && !strings.HasPrefix(resolved, "data:") {
				http.Redirect(w, r, resolved, http.StatusTemporaryRedirect)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) serveGroupImage(w http.ResponseWriter, r *http.Request, groupID string) {
	if s.groups != nil && s.logoCache != nil && s.channels != nil {
		channels, _ := s.channels.List(r.Context())
		for _, ch := range channels {
			if ch.GroupID == groupID && ch.LogoURL != "" {
				resolved := s.logoCache.Resolve(ch.LogoURL)
				if resolved != "" && !strings.HasPrefix(resolved, "data:") {
					http.Redirect(w, r, resolved, http.StatusTemporaryRedirect)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) serveSeriesImage(w http.ResponseWriter, r *http.Request, seriesID string, isBackdrop bool) {
	if s.streams == nil || s.tmdbCache == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	streams, _ := s.streams.List(r.Context())
	for _, st := range streams {
		if st.VODType != "series" {
			continue
		}
		key := seriesKeyFromStream(&st)
		if seriesIDFromName(key) != seriesID {
			continue
		}

		if isBackdrop {
			bd := s.lookupBackdrop(key, "series")
			if bd != "" {
				s.serveTMDBImage(w, r, "w1280", bd)
				return
			}
		}

		poster := s.lookupPoster(key, "tv")
		if poster != "" {
			s.serveTMDBImage(w, r, "w500", poster)
			return
		}
		break
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) serveTMDBImage(w http.ResponseWriter, r *http.Request, size, path string) {
	if s.imageCache != nil {
		r2 := r.Clone(r.Context())
		q := r2.URL.Query()
		q.Set("size", size)
		q.Set("path", path)
		r2.URL.RawQuery = q.Encode()
		s.imageCache.ServeHTTP(w, r2)
		return
	}

	imageURL := tmdb.ImageURL(path, size)
	http.Redirect(w, r, imageURL, http.StatusTemporaryRedirect)
}

func (s *Server) servePersonImage(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("personId")
	if personID == "" {
		personID = r.PathValue("itemId")
	}
	tmdbIDStr := strings.TrimPrefix(personID, "person_")
	if tmdbIDStr == "" || tmdbIDStr == personID {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if s.tmdbClient != nil {
		profilePath := s.tmdbClient.PersonProfilePath(tmdbIDStr)
		if profilePath != "" {
			s.serveTMDBImage(w, r, "w185", profilePath)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func lookupSeriesNameByHash(s *Server, r *http.Request, h uint32) string {
	if s.streams == nil {
		return ""
	}
	streams, _ := s.streams.List(r.Context())
	for _, st := range streams {
		if st.VODType != "series" {
			continue
		}
		key := seriesKeyFromStream(&st)
		if hashString(key) == h {
			return key
		}
	}
	return ""
}
