package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleOutputM3U(w http.ResponseWriter, r *http.Request) {
	baseURL := httputil.RequestBaseURL(r)

	channels, err := s.deps.ChannelStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	groups, err := s.deps.GroupStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list groups")
		return
	}
	groupNames := make(map[string]string, len(groups))
	for _, g := range groups {
		groupNames[g.ID] = g.Name
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n")

	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}

		b.WriteString("#EXTINF:-1")

		tvgID := ch.TvgID
		if tvgID == "" {
			tvgID = fmt.Sprintf("mediahub.%s", ch.ID)
		}
		b.WriteString(fmt.Sprintf(` tvg-id="%s"`, xmlEscape(tvgID)))
		b.WriteString(fmt.Sprintf(` tvg-name="%s"`, xmlEscape(ch.Name)))

		if ch.LogoURL != "" {
			b.WriteString(fmt.Sprintf(` tvg-logo="%s"`, xmlEscape(ch.LogoURL)))
		}

		if ch.GroupID != "" {
			if name, ok := groupNames[ch.GroupID]; ok {
				b.WriteString(fmt.Sprintf(` group-title="%s"`, xmlEscape(name)))
			}
		}

		b.WriteString(fmt.Sprintf(",%s\n", ch.Name))
		b.WriteString(fmt.Sprintf("%s/channel/%s\n", baseURL, ch.ID))
	}

	w.Header().Set("Content-Type", "audio/x-mpegurl")
	w.Header().Set("Content-Disposition", `attachment; filename="playlist.m3u"`)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(b.String()))
}

func (s *Server) handleOutputEPG(w http.ResponseWriter, r *http.Request) {
	channels, err := s.deps.ChannelStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	programs, err := s.deps.ProgramStore.ListAll(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list programs")
		return
	}

	programsByChannel := make(map[string]bool)
	for _, p := range programs {
		programsByChannel[p.ChannelID] = true
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE tv SYSTEM "xmltv.dtd">` + "\n")
	b.WriteString(`<tv generator-info-name="mediahub">` + "\n")

	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}

		tvgID := ch.TvgID
		if tvgID == "" {
			tvgID = fmt.Sprintf("mediahub.%s", ch.ID)
		}

		b.WriteString(fmt.Sprintf(`  <channel id="%s">`, xmlEscape(tvgID)))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf(`    <display-name>%s</display-name>`, xmlEscape(ch.Name)))
		b.WriteString("\n")
		if ch.LogoURL != "" {
			b.WriteString(fmt.Sprintf(`    <icon src="%s" />`, xmlEscape(ch.LogoURL)))
			b.WriteString("\n")
		}
		b.WriteString("  </channel>\n")
	}

	enabledTvgIDs := make(map[string]bool)
	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		if ch.TvgID != "" {
			enabledTvgIDs[ch.TvgID] = true
		}
	}

	for _, p := range programs {
		if !enabledTvgIDs[p.ChannelID] {
			continue
		}

		start := p.StartTime.Format("20060102150405 -0700")
		stop := p.EndTime.Format("20060102150405 -0700")

		b.WriteString(fmt.Sprintf(`  <programme start="%s" stop="%s" channel="%s">`,
			start, stop, xmlEscape(p.ChannelID)))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf(`    <title>%s</title>`, xmlEscape(p.Title)))
		b.WriteString("\n")

		if p.Subtitle != "" {
			b.WriteString(fmt.Sprintf(`    <sub-title>%s</sub-title>`, xmlEscape(p.Subtitle)))
			b.WriteString("\n")
		}
		if p.Description != "" {
			b.WriteString(fmt.Sprintf(`    <desc>%s</desc>`, xmlEscape(p.Description)))
			b.WriteString("\n")
		}
		if len(p.Categories) > 0 {
			for _, cat := range p.Categories {
				b.WriteString(fmt.Sprintf(`    <category>%s</category>`, xmlEscape(cat)))
				b.WriteString("\n")
			}
		}
		if p.Rating != "" {
			b.WriteString("    <rating>\n")
			b.WriteString(fmt.Sprintf(`      <value>%s</value>`, xmlEscape(p.Rating)))
			b.WriteString("\n")
			b.WriteString("    </rating>\n")
		}
		if p.EpisodeNum != "" {
			b.WriteString(fmt.Sprintf(`    <episode-num system="onscreen">%s</episode-num>`, xmlEscape(p.EpisodeNum)))
			b.WriteString("\n")
		}
		if p.IsNew {
			b.WriteString("    <new />\n")
		}

		b.WriteString("  </programme>\n")
	}

	b.WriteString("</tv>\n")

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", `attachment; filename="epg.xml"`)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(b.String()))
}

func (s *Server) handleChannelStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	ch, err := s.deps.ChannelStore.Get(r.Context(), id)
	if err != nil || ch == nil {
		http.NotFound(w, r)
		return
	}

	if len(ch.StreamIDs) == 0 {
		httputil.RespondError(w, http.StatusNotFound, "channel has no streams")
		return
	}

	stream, err := s.deps.StreamStore.Get(r.Context(), ch.StreamIDs[0])
	if err != nil || stream == nil {
		httputil.RespondError(w, http.StatusNotFound, "stream not found")
		return
	}

	http.Redirect(w, r, stream.URL, http.StatusFound)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
