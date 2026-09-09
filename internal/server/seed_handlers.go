package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"jukeboks/internal/config"
	"jukeboks/internal/ytmd"
)

func (s *Server) registerSeedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/seed/status", s.seedStatusHandler)
	mux.HandleFunc("POST /api/seed/resolve", s.seedResolveHandler)
	mux.HandleFunc("POST /api/seed/playlists", s.seedAddPlaylistHandler)
	mux.HandleFunc("DELETE /api/seed/playlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.seedDeletePlaylistHandler(w, r, r.PathValue("id"))
	})
	mux.HandleFunc("POST /api/seed/settings", s.seedSettingsHandler)
	mux.HandleFunc("POST /cmd/jb/seed/enqueue", s.seedEnqueueHandler)
	mux.HandleFunc("DELETE /cmd/jb/seed/queue", s.seedRemoveFromQueueHandler)
}

func (s *Server) apiNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(Envelope{
		ExitCode: 1,
		Message:  fmt.Sprintf("unknown API route: %s (restart jukeboks if this endpoint should exist)", r.URL.Path),
	})
}

func (s *Server) seedStatusHandler(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Config.Get()
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to load config: %v", err)})
		return
	}

	if refreshed, err := s.refreshSeedPlaylistNames(r.Context(), &cfg); err == nil && refreshed {
		_ = s.Config.Save(cfg)
	}

	active, videoCount, playlistCount := s.Seed.Snapshot()
	playing := "none"
	if songPayload, err := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/song", 2, 100*time.Millisecond); err == nil {
		playing = s.Seed.ClassifyVideoID(ytmd.CurrentSongVideoID(songPayload))
	}

	writeJSON(w, Envelope{
		ExitCode: 0,
		Data: map[string]any{
			"seedModeActive":          active,
			"activeSeedVideoCount":    videoCount,
			"activeSeedPlaylistCount": playlistCount,
			"activeRequestCount":      s.Seed.RequestCount(),
			"playingKind":             playing,
			"clearQueueOnRequest":     cfg.ClearQueueOnRequest,
			"seedPlaylists":           cfg.SeedPlaylists,
		},
	})
}

func (s *Server) seedResolveHandler(w http.ResponseWriter, r *http.Request) {
	input, err := seedInputFromRequest(r)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: err.Error()})
		return
	}

	playlistID, err := ytmd.ExtractPlaylistID(input)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: err.Error()})
		return
	}

	lookup, err := ytmd.LookupPlaylistByID(r.Context(), s.YTMD, playlistID)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to resolve playlist: %v", err)})
		return
	}

	writeJSON(w, Envelope{
		ExitCode: 0,
		Message:  fmt.Sprintf("Resolved playlist: %s", lookup.Name),
		Data: map[string]any{
			"id":         lookup.ID,
			"name":       lookup.Name,
			"trackCount": len(lookup.Tracks),
		},
	})
}

func (s *Server) seedAddPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	input, err := seedInputFromRequest(r)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: err.Error()})
		return
	}

	playlistID, err := ytmd.ExtractPlaylistID(input)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: err.Error()})
		return
	}

	lookup, err := ytmd.LookupPlaylistByID(r.Context(), s.YTMD, playlistID)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to resolve playlist: %v", err)})
		return
	}

	playlist := config.SeedPlaylist{
		ID:         lookup.ID,
		Name:       lookup.Name,
		TrackCount: len(lookup.Tracks),
	}
	if err := s.updateSeedPlaylistInConfig(playlist); err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to save playlist: %v", err)})
		return
	}

	writeJSON(w, Envelope{
		ExitCode: 0,
		Message:  fmt.Sprintf("Saved seed playlist: %s", lookup.Name),
		Data:     playlist,
	})
}

func (s *Server) seedDeletePlaylistHandler(w http.ResponseWriter, r *http.Request, playlistID string) {
	playlistID = strings.TrimSpace(playlistID)
	if playlistID == "" {
		writeJSON(w, Envelope{ExitCode: 1, Message: "missing playlist id"})
		return
	}
	if err := s.removeSeedPlaylistFromConfig(playlistID); err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to remove playlist: %v", err)})
		return
	}
	writeJSON(w, Envelope{ExitCode: 0, Message: "Seed playlist removed"})
}

func (s *Server) seedSettingsHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClearQueueOnRequest *bool `json:"clearQueueOnRequest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to decode settings: %v", err)})
		return
	}
	if body.ClearQueueOnRequest == nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: "missing clearQueueOnRequest"})
		return
	}

	cfg, err := s.Config.Get()
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to load config: %v", err)})
		return
	}
	cfg.ClearQueueOnRequest = *body.ClearQueueOnRequest
	if err := s.Config.Save(cfg); err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to save settings: %v", err)})
		return
	}

	writeJSON(w, Envelope{
		ExitCode: 0,
		Message:  "Seed settings saved",
		Data: map[string]any{
			"clearQueueOnRequest": cfg.ClearQueueOnRequest,
		},
	})
}

func (s *Server) seedEnqueueHandler(w http.ResponseWriter, r *http.Request) {
	playlistID := strings.TrimSpace(r.URL.Query().Get("playlistId"))
	if playlistID == "" {
		writeJSON(w, Envelope{ExitCode: 1, Message: "missing playlistId parameter"})
		return
	}

	if _, ok := s.findSavedSeedPlaylist(playlistID); !ok {
		writeJSON(w, Envelope{ExitCode: 1, Message: "playlist is not in saved seed playlists"})
		return
	}

	cfg, err := s.Config.Get()
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to load config: %v", err)})
		return
	}

	lookup, err := ytmd.LookupPlaylistByID(r.Context(), s.YTMD, playlistID)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to look up playlist: %v", err)})
		return
	}

	beforeIDs := map[string]struct{}{}
	if queuePayload, queueErr := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/queue", 2, 250*time.Millisecond); queueErr == nil {
		for _, id := range ytmd.QueueItemVideoIDs(queuePayload) {
			beforeIDs[strings.ToLower(strings.TrimSpace(id))] = struct{}{}
		}
	}

	added, skipped, videoIDs, err := s.enqueueSeedTracks(r.Context(), cfg, lookup)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to enqueue seed playlist: %v", err)})
		return
	}
	if added == 0 {
		writeJSON(w, Envelope{ExitCode: 1, Message: "no tracks were added to the queue"})
		return
	}

	s.Seed.RegisterEnqueue(lookup.ID, videoIDs)
	if queuePayload, queueErr := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/queue", 3, 250*time.Millisecond); queueErr == nil {
		liveIDs := make([]string, 0)
		for _, id := range ytmd.QueueItemVideoIDs(queuePayload) {
			normalized := strings.ToLower(strings.TrimSpace(id))
			if normalized == "" {
				continue
			}
			if _, existed := beforeIDs[normalized]; !existed {
				liveIDs = append(liveIDs, normalized)
			}
		}
		if len(liveIDs) > 0 {
			s.Seed.RegisterEnqueue(lookup.ID, liveIDs)
		}
	}
	_ = s.updateSeedPlaylistInConfig(config.SeedPlaylist{
		ID:         lookup.ID,
		Name:       lookup.Name,
		TrackCount: len(lookup.Tracks),
	})

	writeJSON(w, Envelope{
		ExitCode: 0,
		Message:  fmt.Sprintf("Added %d tracks from %s to queue", added, lookup.Name),
		Data: map[string]any{
			"playlistId":     lookup.ID,
			"name":           lookup.Name,
			"added":          added,
			"skipped":        skipped,
			"trackCount":     len(lookup.Tracks),
			"seedModeActive": true,
		},
	})
}

func (s *Server) seedRemoveFromQueueHandler(w http.ResponseWriter, r *http.Request) {
	playlistID := strings.TrimSpace(r.URL.Query().Get("playlistId"))
	if playlistID == "" {
		writeJSON(w, Envelope{ExitCode: 1, Message: "missing playlistId parameter"})
		return
	}

	lookup, err := ytmd.LookupPlaylistByID(r.Context(), s.YTMD, playlistID)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to look up playlist: %v", err)})
		return
	}

	target := map[string]struct{}{}
	for _, track := range lookup.Tracks {
		target[strings.ToLower(strings.TrimSpace(track.VideoID))] = struct{}{}
	}

	queuePayload, err := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/queue", 3, 250)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach queue endpoint: %v", err)})
		return
	}

	indices := ytmd.QueueIndicesForVideoIDs(queuePayload, target)
	if err := s.deleteQueueIndices(r.Context(), indices); err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to remove seed tracks from queue: %v", err)})
		return
	}

	queuePayload, _ = s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/queue", 2, 250)
	songPayload, _ := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/song", 2, 250)
	s.Seed.SyncFromQueue(queuePayload, ytmd.CurrentSongVideoID(songPayload))

	writeJSON(w, Envelope{
		ExitCode: 0,
		Message:  fmt.Sprintf("Removed %d seed tracks from queue", len(indices)),
		Data: map[string]any{
			"removed":        len(indices),
			"seedModeActive": s.Seed.IsActive(),
		},
	})
}

func seedInputFromRequest(r *http.Request) (string, error) {
	if value := strings.TrimSpace(r.URL.Query().Get("input")); value != "" {
		return value, nil
	}

	var body struct {
		Input string `json:"input"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if value := strings.TrimSpace(body.Input); value != "" {
				return value, nil
			}
		}
	}

	return "", fmt.Errorf("missing input")
}
