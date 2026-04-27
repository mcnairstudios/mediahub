package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
	"github.com/mcnairstudios/mediahub/pkg/recording"
)

func (s *Server) handleScheduleRecording(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ChannelID string    `json:"channel_id"`
		Title     string    `json:"title"`
		StartAt   time.Time `json:"start_at"`
		StopAt    time.Time `json:"stop_at"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ChannelID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "channel_id required")
		return
	}
	if req.Title == "" {
		httputil.RespondError(w, http.StatusBadRequest, "title required")
		return
	}
	if req.StartAt.IsZero() || req.StopAt.IsZero() {
		httputil.RespondError(w, http.StatusBadRequest, "start_at and stop_at required")
		return
	}
	if !req.StopAt.After(req.StartAt) {
		httputil.RespondError(w, http.StatusBadRequest, "stop_at must be after start_at")
		return
	}

	ch, err := s.deps.ChannelStore.Get(r.Context(), req.ChannelID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to look up channel")
		return
	}
	if ch == nil {
		httputil.RespondError(w, http.StatusNotFound, "channel not found")
		return
	}

	var streamID string
	if len(ch.StreamIDs) > 0 {
		streamID = ch.StreamIDs[0]
	}
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "channel has no assigned streams")
		return
	}

	b := make([]byte, 16)
	rand.Read(b)
	recID := hex.EncodeToString(b)

	rec := &recording.Recording{
		ID:             recID,
		StreamID:       streamID,
		ChannelID:      req.ChannelID,
		ChannelName:    ch.Name,
		Title:          req.Title,
		UserID:         user.ID,
		Status:         recording.StatusScheduled,
		ScheduledStart: req.StartAt,
		ScheduledStop:  req.StopAt,
	}

	if err := s.deps.RecordingStore.Create(r.Context(), rec); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to schedule recording")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleListScheduledRecordings(w http.ResponseWriter, r *http.Request) {
	scheduled, err := s.deps.RecordingStore.ListScheduled(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list scheduled recordings")
		return
	}

	active, err := s.deps.RecordingStore.ListByStatus(r.Context(), recording.StatusRecording)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list active recordings")
		return
	}

	all := append(scheduled, active...)
	httputil.RespondJSON(w, http.StatusOK, all)
}

func (s *Server) handleCancelScheduledRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	rec, err := s.deps.RecordingStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to look up recording")
		return
	}
	if rec == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if rec.Status != recording.StatusScheduled {
		httputil.RespondError(w, http.StatusConflict, "can only cancel scheduled recordings")
		return
	}

	if err := s.deps.RecordingStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to cancel recording")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
