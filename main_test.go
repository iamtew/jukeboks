package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYTMDRouteMapping(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		expected string
	}{
		{name: "play", path: "/cmd/ytmd/play", expected: "/api/v1/play"},
		{name: "song", path: "/cmd/ytmd/song", expected: "/api/v1/song"},
		{name: "queue next", path: "/cmd/ytmd/queue/next", expected: "/api/v1/queue/next"},
		{name: "root", path: "/cmd/ytmd/", expected: "/api/v1/"},
		{name: "root without slash", path: "/cmd/ytmd", expected: "/api/v1/"},
		{name: "queue with method suffix", path: "/cmd/ytmd/queue/post", expected: "/api/v1/queue"},
		{name: "search with method suffix", path: "/cmd/ytmd/search/post", expected: "/api/v1/search"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ytmdRouteForPath(tc.path); got != tc.expected {
				t.Fatalf("ytmdRouteForPath(%q) = %q, want %q", tc.path, got, tc.expected)
			}
		})
	}
}

func TestMethodSuffixResolution(t *testing.T) {
	cases := []struct {
		name          string
		path          string
		requestMethod string
		expected      string
	}{
		{name: "queue post suffix", path: "/cmd/ytmd/queue/post", requestMethod: "GET", expected: "POST"},
		{name: "queue patch suffix", path: "/cmd/ytmd/queue/patch", requestMethod: "GET", expected: "PATCH"},
		{name: "queue delete suffix", path: "/cmd/ytmd/queue/delete", requestMethod: "GET", expected: "DELETE"},
		{name: "search post suffix", path: "/cmd/ytmd/search/post", requestMethod: "GET", expected: "POST"},
		{name: "plain play route", path: "/cmd/ytmd/play", requestMethod: "GET", expected: "POST"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveUpstreamMethod(tc.requestMethod, tc.path); got != tc.expected {
				t.Fatalf("resolveUpstreamMethod(%q, %q) = %q, want %q", tc.requestMethod, tc.path, got, tc.expected)
			}
		})
	}
}

func TestBuildUpstreamBodyUsesRequestBodyForPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/seek-to/post?seconds=42", bytes.NewBufferString(`{"seconds": 7}`))
	bodyReader, err := buildUpstreamBody(http.MethodPost, req)
	if err != nil {
		t.Fatalf("buildUpstreamBody returned error: %v", err)
	}

	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}

	if string(bodyBytes) != `{"seconds": 7}` {
		t.Fatalf("buildUpstreamBody() = %q, want JSON body", string(bodyBytes))
	}
}

func TestBuildUpstreamBodyFallsBackToQueryForPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/seek-to/post?seconds=42", nil)
	bodyReader, err := buildUpstreamBody(http.MethodPost, req)
	if err != nil {
		t.Fatalf("buildUpstreamBody returned error: %v", err)
	}

	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}

	if string(bodyBytes) != `{"seconds":42}` {
		t.Fatalf("buildUpstreamBody() = %q, want numeric query payload", string(bodyBytes))
	}
}

func TestIsNormalShutdownError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "server closed", err: http.ErrServerClosed, want: true},
		{name: "context canceled", err: context.Canceled, want: true},
		{name: "net closed", err: net.ErrClosed, want: true},
		{name: "use of closed network connection", err: errors.New("use of closed network connection"), want: true},
		{name: "other error", err: errors.New("boom"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNormalShutdownError(tc.err); got != tc.want {
				t.Fatalf("isNormalShutdownError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestEnforceConfigPolicyBlocksBlacklistedContent(t *testing.T) {
	cfg := jukeboksConfig{Blacklist: []string{"rick astley", "never gonna give you up"}, MaxDuration: 600}
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/queue/post?query=Never%20Gonna%20Give%20You%20Up", nil)

	if err := enforceConfigPolicy(cfg, req); err == nil {
		t.Fatal("enforceConfigPolicy() error = nil, want blacklist rejection")
	}
}

func TestEnforceConfigPolicyBlocksBlacklistedJSONBody(t *testing.T) {
	cfg := jukeboksConfig{Blacklist: []string{"never gonna give you up"}, MaxDuration: 600}
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/queue/post", strings.NewReader(`{"title":"Never Gonna Give You Up"}`))

	if err := enforceConfigPolicy(cfg, req); err == nil {
		t.Fatal("enforceConfigPolicy() error = nil, want JSON body blacklist rejection")
	}
}

func TestEnforceConfigPolicyBlocksOverlongContent(t *testing.T) {
	cfg := jukeboksConfig{Blacklist: []string{"Rick Astley"}, MaxDuration: 600}
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/queue/post?duration=900", nil)

	if err := enforceConfigPolicy(cfg, req); err == nil {
		t.Fatal("enforceConfigPolicy() error = nil, want maxDuration rejection")
	}
}

func TestBuildQueueInfoResponseWhenPlaying(t *testing.T) {
	songPayload := map[string]any{
		"song":         map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused":     false,
		"position":     120,
		"songDuration": 240,
	}
	queuePayload := map[string]any{
		"items": []any{
			map[string]any{"playlistPanelVideoRenderer": map[string]any{"title": map[string]any{"runs": []any{map[string]any{"text": "Song A"}}}, "longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist A"}}}, "lengthText": map[string]any{"runs": []any{map[string]any{"text": "3:20"}}}}},
			map[string]any{"playlistPanelVideoRenderer": map[string]any{"title": map[string]any{"runs": []any{map[string]any{"text": "Song B"}}}, "longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist B"}}}, "lengthText": map[string]any{"runs": []any{map[string]any{"text": "2:10"}}}}},
			map[string]any{"playlistPanelVideoRenderer": map[string]any{"title": map[string]any{"runs": []any{map[string]any{"text": "Song C"}}}, "longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist C"}}}, "lengthText": map[string]any{"runs": []any{map[string]any{"text": "1:05"}}}}},
			map[string]any{"playlistPanelVideoRenderer": map[string]any{"title": map[string]any{"runs": []any{map[string]any{"text": "Song D"}}}, "longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist D"}}}, "lengthText": map[string]any{"runs": []any{map[string]any{"text": "2:00"}}}}},
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}
	if !strings.Contains(resp.Message, "Queue:") {
		t.Fatalf("message = %q, want queue prefix", resp.Message)
	}
	if !strings.Contains(resp.Message, "totaling") {
		t.Fatalf("message = %q, want total-duration summary", resp.Message)
	}
	if !strings.Contains(resp.Message, "08:35") {
		t.Fatalf("message = %q, want total duration summary", resp.Message)
	}
	if resp.Data == nil {
		t.Fatal("expected queue summary payload in data")
	}
}

func TestBuildQueueInfoResponseWhenPaused(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused": true,
	}
	queuePayload := map[string]any{"items": []any{}}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode == 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = 0, want non-zero for paused player")
	}
	if !strings.Contains(strings.ToLower(resp.Message), "queue unavailable") {
		t.Fatalf("message = %q, want queue unavailable guidance", resp.Message)
	}
}

func TestBuildSongInfoResponseWhenPlaying(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused": false,
	}

	resp := buildSongInfoResponse(songPayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildSongInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}
	if !strings.Contains(resp.Message, "Song:") {
		t.Fatalf("message = %q, want song prefix", resp.Message)
	}
	if !strings.Contains(resp.Message, "Playback state: playing") {
		t.Fatalf("message = %q, want playback state", resp.Message)
	}
}

func TestBuildSongInfoResponseWhenPaused(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused": true,
	}

	resp := buildSongInfoResponse(songPayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildSongInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}
	if !strings.Contains(resp.Message, "Playback state: paused") {
		t.Fatalf("message = %q, want paused state", resp.Message)
	}
}

func TestEnsureConfigFileCreatesDefaultConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "jukeboks.json")

	cfg, err := ensureConfigFile(configPath)
	if err != nil {
		t.Fatalf("ensureConfigFile returned error: %v", err)
	}

	if len(cfg.Blacklist) != 1 || cfg.Blacklist[0] != "Rick Astley" {
		t.Fatalf("default blacklist = %#v, want [Rick Astley]", cfg.Blacklist)
	}
	if cfg.MaxDuration != 600 {
		t.Fatalf("default maxDuration = %d, want 600", cfg.MaxDuration)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
}

func TestSaveConfigPersistsToDisk(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "jukeboks.json")

	cfg, err := ensureConfigFile(configPath)
	if err != nil {
		t.Fatalf("ensureConfigFile returned error: %v", err)
	}

	cfg.Blacklist = []string{"Artist A", "Artist B"}
	cfg.MaxDuration = 900
	if err := saveConfig(configPath, cfg); err != nil {
		t.Fatalf("saveConfig returned error: %v", err)
	}

	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if len(loaded.Blacklist) != 2 || loaded.Blacklist[0] != "Artist A" || loaded.Blacklist[1] != "Artist B" {
		t.Fatalf("loaded blacklist = %#v, want [Artist A Artist B]", loaded.Blacklist)
	}
	if loaded.MaxDuration != 900 {
		t.Fatalf("loaded maxDuration = %d, want 900", loaded.MaxDuration)
	}
}
