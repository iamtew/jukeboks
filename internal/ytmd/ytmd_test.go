package ytmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
			if got := RouteForPath(tc.path); got != tc.expected {
				t.Fatalf("RouteForPath(%q) = %q, want %q", tc.path, got, tc.expected)
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
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/seek-to?seconds=42", bytes.NewBufferString(`{"seconds": 7}`))
	bodyReader, err := BuildUpstreamBody(http.MethodPost, req)
	if err != nil {
		t.Fatalf("buildUpstreamBody returned error: %v", err)
	}

	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}
	if string(bodyBytes) != `{"seconds": 7}` {
		t.Fatalf("BuildUpstreamBody() = %q, want JSON body", string(bodyBytes))
	}
}

func TestBuildUpstreamBodyFallsBackToQueryForPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/seek-to?seconds=42", nil)
	bodyReader, err := BuildUpstreamBody(http.MethodPost, req)
	if err != nil {
		t.Fatalf("buildUpstreamBody returned error: %v", err)
	}

	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}
	if string(bodyBytes) != `{"seconds":42}` {
		t.Fatalf("BuildUpstreamBody() = %q, want numeric query payload", string(bodyBytes))
	}
}

func TestFetchJSONWithRetryEventuallySucceeds(t *testing.T) {
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

func TestBuildSongInfoResponseWhenPlaying(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused": false,
	}

	resp := BuildSongInfoResponse(songPayload)
	if resp.ExitCode != 0 {
		t.Fatalf("BuildSongInfoResponse() exitCode = %d, want 0", resp.ExitCode)
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

	resp := BuildSongInfoResponse(songPayload)
	if resp.ExitCode != 0 {
		t.Fatalf("BuildSongInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}
	if !strings.Contains(resp.Message, "Playback state: paused") {
		t.Fatalf("message = %q, want paused state", resp.Message)
	}
}
