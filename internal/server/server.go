package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"jukeboks/internal/config"
	"jukeboks/internal/policy"
	"jukeboks/internal/seed"
	"jukeboks/internal/ytmd"
)

type Envelope struct {
	ExitCode int    `json:"exitCode"`
	Message  string `json:"message,omitempty"`
	Data     any    `json:"data,omitempty"`
}

type Server struct {
	Config  *config.Store
	YTMD    *ytmd.Client
	Webroot string
	Seed    *seed.State
}

func New(cfg *config.Store, client *ytmd.Client, webroot string) *Server {
	return &Server{Config: cfg, YTMD: client, Webroot: webroot, Seed: seed.NewState()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, Envelope{
			ExitCode: 0,
			Message:  "ok",
			Data: map[string]any{
				"features": []string{"seed"},
			},
		})
	})

	mux.HandleFunc("/cmd/ytmd/", s.proxyToYTMD)
	mux.HandleFunc("/cmd/ytmd", s.proxyToYTMD)

	mux.HandleFunc("/cmd/jb/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cmd/jb/queueinfo", "/cmd/jb/queueinfo/":
			s.queueInfoHandler(w, r)
			return
		case "/cmd/jb/songinfo", "/cmd/jb/songinfo/":
			s.songInfoHandler(w, r)
			return
		case "/cmd/jb/songrequest", "/cmd/jb/songrequest/":
			s.songRequestHandler(w, r)
			return
		}
		writeJSON(w, Envelope{ExitCode: 0, Message: "custom jukeboks command scaffold"})
	})

	mux.HandleFunc("/api/config", s.configHandler)
	s.registerSeedRoutes(mux)

	mux.HandleFunc("/api/", s.apiNotFoundHandler)

	mux.Handle("/", http.FileServer(http.Dir(s.Webroot)))

	return loggingMiddleware(mux)
}

func (s *Server) configHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := s.Config.Get()
		if err != nil {
			writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to load config: %v", err)})
			return
		}
		writeJSON(w, Envelope{ExitCode: 0, Data: cfg})
	case http.MethodPost, http.MethodPut:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to decode config: %v", err)})
			return
		}
		if err := s.Config.Save(cfg); err != nil {
			writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to save config: %v", err)})
			return
		}
		saved, err := s.Config.Get()
		if err != nil {
			writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to load saved config: %v", err)})
			return
		}
		writeJSON(w, Envelope{ExitCode: 0, Message: "config saved", Data: saved})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) songInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, Envelope{ExitCode: 1, Message: "songinfo requires GET"})
		return
	}

	songPayload, err := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/song", 3, 250*time.Millisecond)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach song endpoint: %v", err)})
		return
	}
	writeJSON(w, ytmd.BuildSongInfoResponse(songPayload))
}

func (s *Server) queueInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, Envelope{ExitCode: 1, Message: "queueinfo requires GET"})
		return
	}

	songPayload, err := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/song", 3, 250*time.Millisecond)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach song endpoint: %v", err)})
		return
	}

	queuePayload, err := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/queue", 3, 250*time.Millisecond)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach queue endpoint: %v", err)})
		return
	}

	writeJSON(w, ytmd.BuildQueueInfoResponse(songPayload, queuePayload))
}

func (s *Server) songRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, Envelope{ExitCode: 1, Message: "songrequest requires GET"})
		return
	}

	input := strings.TrimSpace(r.URL.Query().Get("input"))
	if input == "" {
		writeJSON(w, Envelope{ExitCode: 1, Message: "missing input parameter"})
		return
	}
	if len(input) > ytmd.MaxSongRequestInputLen {
		writeJSON(w, Envelope{ExitCode: 1, Message: "input too long"})
		return
	}

	videoID, urlLike := ytmd.ClassifySongRequestInput(input)
	var lookup ytmd.SongLookup
	var err error
	if videoID != "" {
		lookup, err = ytmd.LookupSongByVideoID(r.Context(), s.YTMD, videoID)
	} else if urlLike {
		writeJSON(w, Envelope{ExitCode: 1, Message: "unsupported or invalid YouTube URL"})
		return
	} else {
		videoID, lookup, err = ytmd.LookupSongByQuery(r.Context(), s.YTMD, input)
	}
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to look up song metadata: %v", err)})
		return
	}

	cfg, err := s.Config.Get()
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to load config: %v", err)})
		return
	}

	if err := policy.CheckContent(cfg, lookup.Duration, true, input, lookup.Title, lookup.Artist); err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: err.Error()})
		return
	}

	queuePayload, err := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/queue", 3, 250*time.Millisecond)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach queue endpoint: %v", err)})
		return
	}

	if ytmd.QueueContainsVideoID(queuePayload, videoID) {
		writeJSON(w, buildDuplicateSongResponse(videoID, lookup))
		return
	}

	songPayload, err := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/song", 3, 250*time.Millisecond)
	if err == nil && strings.EqualFold(ytmd.CurrentSongVideoID(songPayload), videoID) {
		writeJSON(w, buildDuplicateSongResponse(videoID, lookup))
		return
	}

	insertPosition := "INSERT_AT_END"
	var queueAfterInsert any
	if s.Seed.IsActive() {
		currentVideoID := ytmd.CurrentSongVideoID(songPayload)
		if cfg.ClearQueueOnRequest {
			seedVideoIDs := s.Seed.VideoIDSet()
			if err := s.removeUpcomingSeedTracks(r.Context(), queuePayload, seedVideoIDs, currentVideoID); err != nil {
				writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to clear seed queue: %v", err)})
				return
			}
			queuePayload, err = s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/queue", 3, 250*time.Millisecond)
			if err != nil {
				writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach queue endpoint: %v", err)})
				return
			}
		}
		queueAfterInsert, err = s.insertRequestDuringSeedMode(r.Context(), queuePayload, songPayload, videoID)
		if err != nil {
			writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach queue endpoint: %v", err)})
			return
		}
	} else {
		queueAfterInsert, err = s.YTMD.PostJSON(r.Context(), "/api/v1/queue", map[string]any{
			"videoId":        videoID,
			"insertPosition": insertPosition,
		})
		if err != nil {
			writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach queue endpoint: %v", err)})
			return
		}
	}

	queuePayload = queueAfterInsert
	if freshQueue, fetchErr := s.YTMD.FetchJSONWithRetry(r.Context(), "/api/v1/queue", 3, 250*time.Millisecond); fetchErr == nil {
		queuePayload = freshQueue
	}

	s.Seed.ReconcileQueue(queuePayload, ytmd.CurrentSongVideoID(songPayload))

	writeJSON(w, ytmd.BuildSongRequestResponse(videoID, queuePayload, lookup))
}

func buildDuplicateSongResponse(videoID string, lookup ytmd.SongLookup) Envelope {
	message := "Song already in queue."
	if lookup.Title != "" && lookup.Artist != "" {
		message = fmt.Sprintf("Song already in queue: %s — %s.", lookup.Title, lookup.Artist)
	} else if lookup.Title != "" {
		message = fmt.Sprintf("Song already in queue: %s.", lookup.Title)
	}

	data := map[string]any{
		"reason":  "duplicate",
		"videoId": videoID,
	}
	if lookup.Title != "" {
		data["title"] = lookup.Title
	}
	if lookup.Artist != "" {
		data["artist"] = lookup.Artist
	}

	return Envelope{ExitCode: 1, Message: message, Data: data}
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
