package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if !strings.Contains(resp.Message, "songs") {
		t.Fatalf("message = %q, want song count summary", resp.Message)
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

func TestFetchYTMDJSONWithRetryEventuallySucceeds(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	payload, err := fetchYTMDJSONWithRetry(target, "/api/v1/queue", 2, 0)
	if err != nil {
		t.Fatalf("fetchYTMDJSONWithRetry() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if payload.(map[string]any)["ok"] != true {
		t.Fatalf("payload = %#v, want ok=true", payload)
	}
}

func TestSummarizeQueueEntryHandlesWrapperRenderer(t *testing.T) {
	entry := summarizeQueueEntry(map[string]any{
		"playlistPanelVideoWrapperRenderer": map[string]any{
			"primaryRenderer": map[string]any{
				"playlistPanelVideoRenderer": map[string]any{
					"title":          map[string]any{"runs": []any{map[string]any{"text": "Overdrive"}}},
					"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Lazerhawk"}}},
					"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "4:32"}}},
				},
			},
		},
	})

	if entry.Title != "Overdrive" {
		t.Fatalf("Title = %q, want Overdrive", entry.Title)
	}
	if entry.Artist != "Lazerhawk" {
		t.Fatalf("Artist = %q, want Lazerhawk", entry.Artist)
	}
}

func TestCollectQueueEntriesForCommandDeduplicatesByTitleAndArtist(t *testing.T) {
	payload := map[string]any{
		"items": []any{
			map[string]any{
				"title":            "Same Song",
				"alternativeTitle": "Same Song",
				"artist":           "Artist A",
				"videoId":          "abc123",
			},
			map[string]any{
				"title":   "Same Song",
				"artist":  "Artist A",
				"videoId": "xyz789",
			},
		},
	}

	entries := collectQueueEntriesForCommand(payload)
	if len(entries) != 1 {
		t.Fatalf("collectQueueEntriesForCommand() returned %d entries, want 1", len(entries))
	}
	if entries[0].Title != "Same Song" {
		t.Fatalf("title = %q, want Same Song", entries[0].Title)
	}
}

func TestBuildQueueInfoResponseStartsFromCurrentSong(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song B", "artist": "Artist B"},
		"isPaused": false,
	}
	queuePayload := map[string]any{
		"items": []any{
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"title":          map[string]any{"runs": []any{map[string]any{"text": "Song A"}}},
				"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist A"}}},
				"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "2:00"}}},
			}},
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"title":          map[string]any{"runs": []any{map[string]any{"text": "Song B"}}},
				"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist B"}}},
				"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "3:00"}}},
			}},
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"title":          map[string]any{"runs": []any{map[string]any{"text": "Song C"}}},
				"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist C"}}},
				"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "4:00"}}},
			}},
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %#v", resp.Data)
	}
	songs, ok := dataMap["songs"].([]queueEntrySummary)
	if !ok || len(songs) != 2 {
		t.Fatalf("expected 2 remaining songs, got %#v", dataMap["songs"])
	}
	if songs[0].Title != "Song B" || songs[1].Title != "Song C" {
		t.Fatalf("queue ordering = %#v, want Song B then Song C", songs)
	}
}

func TestBuildQueueInfoResponseUsesVideoIDToStartFromCurrentSong(t *testing.T) {
	songPayload := map[string]any{
		"song": map[string]any{
			"title":   "Not The Queue Title",
			"artist":  "Different Artist",
			"videoId": "video-2",
		},
		"isPaused": false,
	}
	queuePayload := map[string]any{
		"items": []any{
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"title":          map[string]any{"runs": []any{map[string]any{"text": "Song A"}}},
				"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist A"}}},
				"videoId":        "video-1",
				"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "2:00"}}},
			}},
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"title":          map[string]any{"runs": []any{map[string]any{"text": "Song B"}}},
				"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist B"}}},
				"videoId":        "video-2",
				"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "3:00"}}},
			}},
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"title":          map[string]any{"runs": []any{map[string]any{"text": "Song C"}}},
				"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Artist C"}}},
				"videoId":        "video-3",
				"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "4:00"}}},
			}},
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %#v", resp.Data)
	}
	songs, ok := dataMap["songs"].([]queueEntrySummary)
	if !ok || len(songs) != 2 {
		t.Fatalf("expected 2 remaining songs, got %#v", dataMap["songs"])
	}
	if songs[0].Title != "Song B" || songs[1].Title != "Song C" {
		t.Fatalf("queue ordering = %#v, want Song B then Song C", songs)
	}
	if totalSeconds, ok := dataMap["totalSeconds"].(int); !ok || totalSeconds != 420 {
		t.Fatalf("totalSeconds = %#v, want 420", dataMap["totalSeconds"])
	}
}

func TestBuildQueueInfoResponseKeepsOnlyArtistFromByline(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused": false,
	}
	queuePayload := map[string]any{
		"items": []any{
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"title":          map[string]any{"runs": []any{map[string]any{"text": "Overdrive"}}},
				"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Lazerhawk"}, map[string]any{"text": "•"}, map[string]any{"text": "Redline"}, map[string]any{"text": "•"}, map[string]any{"text": "2010"}}},
				"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "4:32"}}},
			}},
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %#v", resp.Data)
	}
	songs, ok := dataMap["songs"].([]queueEntrySummary)
	if !ok || len(songs) != 1 {
		t.Fatalf("expected one queue entry, got %#v", dataMap["songs"])
	}
	if songs[0].Artist != "Lazerhawk" {
		t.Fatalf("artist = %q, want Lazerhawk", songs[0].Artist)
	}
	if strings.Contains(songs[0].DisplayText, "Redline") || strings.Contains(songs[0].DisplayText, "2010") {
		t.Fatalf("display text = %q, should not include album or year", songs[0].DisplayText)
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

func TestLoadConfigNormalizesMalformedValues(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "jukeboks.json")

	contents := []byte(`{"blacklist":["  ","Rick Astley",""],"maxDuration":-1}`)
	if err := os.WriteFile(configPath, contents, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if len(loaded.Blacklist) != 1 || loaded.Blacklist[0] != "Rick Astley" {
		t.Fatalf("loaded blacklist = %#v, want [Rick Astley]", loaded.Blacklist)
	}
	if loaded.MaxDuration != 600 {
		t.Fatalf("loaded maxDuration = %d, want 600", loaded.MaxDuration)
	}
}
