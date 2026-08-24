package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"jukeboks/internal/config"
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
}

func New(cfg *config.Store, client *ytmd.Client, webroot string) *Server {
	return &Server{Config: cfg, YTMD: client, Webroot: webroot}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, Envelope{ExitCode: 0, Message: "ok"})
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
		}
		writeJSON(w, Envelope{ExitCode: 0, Message: "custom jukeboks command scaffold"})
	})

	mux.HandleFunc("/api/config", s.configHandler)

	mux.Handle("/", http.FileServer(http.Dir(s.Webroot)))

	return mux
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

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
