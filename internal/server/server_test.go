package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jukeboks/internal/config"
	"jukeboks/internal/httplog"
	"jukeboks/internal/ytmd"
)

func newTestEnv(t *testing.T, ytmdHandler http.Handler) (*httptest.Server, *httptest.Server, string) {
	t.Helper()

	httplog.SetEnabled(false)
	t.Cleanup(func() {
		httplog.SetEnabled(true)
	})

	upstream := httptest.NewServer(ytmdHandler)
	t.Cleanup(upstream.Close)

	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	dir := t.TempDir()
	webroot := filepath.Join(dir, "webroot")
	if err := os.MkdirAll(filepath.Join(webroot, "admin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(webroot, "overlay"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webroot, "index.html"), []byte("landing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webroot, "admin", "index.html"), []byte("admin-html"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webroot, "overlay", "index.html"), []byte("overlay-html"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := config.NewStore(filepath.Join(dir, "jukeboks.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	srv := New(store, ytmd.NewClient(target), webroot)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, upstream, dir
}

func decodeEnvelope(t *testing.T, body []byte) Envelope {
	t.Helper()
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("Unmarshal() error = %v body = %s", err, body)
	}
	return env
}

const testSearchResponse = `{"contents":{"tabbedSearchResultsRenderer":{"tabs":[{"tabRenderer":{"content":{"sectionListRenderer":{"contents":[{"musicResponsiveListItemRenderer":{"videoId":"abc12345678","title":{"runs":[{"text":"Test Song"}]},"longBylineText":{"runs":[{"text":"Test Artist"}]},"lengthText":{"runs":[{"text":"3:30"}]}}}]}}}}]}}}`

const blockedSearchResponse = `{"contents":{"tabbedSearchResultsRenderer":{"tabs":[{"tabRenderer":{"content":{"sectionListRenderer":{"contents":[{"musicCardShelfRenderer":{"title":{"runs":[{"text":"Never Gonna Give You Up"}]},"subtitle":{"runs":[{"text":"Video"},{"text":" • "},{"text":"Rick Astley"},{"text":" • "},{"text":"3:34"}]},"onTap":{"watchEndpoint":{"videoId":"dQw4w9WgXcQ"}}}}]}}}}]}}}`

const emptyQueueResponse = `{"items":[]}`

const duplicateQueueResponse = `{"items":[{"playlistPanelVideoRenderer":{"videoId":"abc12345678","title":{"runs":[{"text":"Test Song"}]},"longBylineText":{"runs":[{"text":"Test Artist"}]},"lengthText":{"runs":[{"text":"3:30"}]}}}]}`

func ytmdSongRequestHandler(t *testing.T, searchResponse, queueResponse string, onQueue func(r *http.Request)) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, searchResponse)
		case "/api/v1/song":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"videoId":"other123456","title":"Other Song","artist":"Other Artist"}`)
		case "/api/v1/queue":
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, queueResponse)
				return
			}
			if onQueue != nil {
				onQueue(r)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected upstream path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestHealthEnvelope(t *testing.T) {
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	env := decodeEnvelope(t, body)
	if env.ExitCode != 0 || env.Message != "ok" {
		t.Fatalf("envelope = %#v, want exitCode 0 message ok", env)
	}
}

func TestConfigGetAndSave(t *testing.T) {
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	resp, err := http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	env := decodeEnvelope(t, body)
	if env.ExitCode != 0 {
		t.Fatalf("get config exitCode = %d, want 0", env.ExitCode)
	}

	payload := `{"blacklist":["No Rick"],"maxDuration":120}`
	resp, err = http.Post(ts.URL+"/api/config", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	saved := decodeEnvelope(t, body)
	if saved.ExitCode != 0 {
		t.Fatalf("save config exitCode = %d message = %q", saved.ExitCode, saved.Message)
	}

	resp, err = http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	reloaded := decodeEnvelope(t, body)
	data, _ := json.Marshal(reloaded.Data)
	if !strings.Contains(string(data), "No Rick") {
		t.Fatalf("reloaded data = %s, want No Rick", data)
	}
}

func TestProxyBlocksBlacklistedCommand(t *testing.T) {
	called := false
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))

	resp, err := http.Post(ts.URL+"/cmd/ytmd/queue?query=Never%20Gonna%20Give%20You%20Up%20Rick%20Astley", "application/json", nil)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1 body = %s", env.ExitCode, body)
	}
	if called {
		t.Fatal("upstream YTMD was called for a blacklisted request")
	}
}

func TestProxyForwardsAllowedCommand(t *testing.T) {
	var seenHost, seenConnection string
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost = r.Host
		seenConnection = r.Header.Get("Connection")
		if r.URL.Path != "/api/v1/play" {
			t.Errorf("upstream path = %q, want /api/v1/play", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/cmd/ytmd/play", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "evil.example"
	req.Header.Set("Connection", "close")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 0 {
		t.Fatalf("exitCode = %d message = %q", env.ExitCode, env.Message)
	}
	if seenHost == "evil.example" {
		t.Fatal("incoming Host was forwarded upstream")
	}
	if seenConnection == "close" {
		t.Fatal("hop-by-hop Connection header was forwarded upstream")
	}
}

func TestStaticAdminAndOverlay(t *testing.T) {
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for _, path := range []string{"/admin/", "/overlay/"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("Get(%s) error = %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, resp.StatusCode)
		}
		if !strings.Contains(string(body), "html") && !strings.Contains(string(body), "admin") && !strings.Contains(string(body), "overlay") {
			t.Fatalf("%s body = %q, want html fixture", path, body)
		}
	}
}

func TestStaticDoesNotEscapeWebroot(t *testing.T) {
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	resp, err := http.Get(ts.URL + "/../secret.txt")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "secret") {
		t.Fatalf("path traversal succeeded, body = %q", body)
	}
}

func TestSongRequestHandlerAddsToQueue(t *testing.T) {
	var seenMethod, seenPath string
	var seenBody map[string]any
	ts, _, _ := newTestEnv(t, ytmdSongRequestHandler(t, testSearchResponse, emptyQueueResponse, func(r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
	}))

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest?input=abc12345678")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 0 {
		t.Fatalf("exitCode = %d message = %q", env.ExitCode, env.Message)
	}
	if seenMethod != http.MethodPost {
		t.Fatalf("upstream method = %q, want POST", seenMethod)
	}
	if seenPath != "/api/v1/queue" {
		t.Fatalf("upstream path = %q, want /api/v1/queue", seenPath)
	}
	if seenBody["videoId"] != "abc12345678" {
		t.Fatalf("upstream videoId = %v, want abc12345678", seenBody["videoId"])
	}
	if seenBody["insertPosition"] != "INSERT_AT_END" {
		t.Fatalf("upstream insertPosition = %v, want INSERT_AT_END", seenBody["insertPosition"])
	}
	if !strings.Contains(env.Message, "Test Song") {
		t.Fatalf("message = %q, want Test Song", env.Message)
	}
}

func TestSongRequestHandlerExtractsFromURL(t *testing.T) {
	var seenVideoID string
	ts, _, _ := newTestEnv(t, ytmdSongRequestHandler(t, testSearchResponse, emptyQueueResponse, func(r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		seenVideoID, _ = body["videoId"].(string)
	}))

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest?input=" + url.QueryEscape("https://youtu.be/abc12345678"))
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 0 {
		t.Fatalf("exitCode = %d message = %q", env.ExitCode, env.Message)
	}
	if seenVideoID != "abc12345678" {
		t.Fatalf("upstream videoId = %q, want abc12345678", seenVideoID)
	}
}

func TestSongRequestHandlerMissingInput(t *testing.T) {
	called := false
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Message, "missing input") {
		t.Fatalf("message = %q, want missing input", env.Message)
	}
	if called {
		t.Fatal("upstream YTMD was called without input")
	}
}

func TestSongRequestHandlerSearchQuery(t *testing.T) {
	var queuedVideoID string
	ts, _, _ := newTestEnv(t, ytmdSongRequestHandler(t, testSearchResponse, emptyQueueResponse, func(r *http.Request) {
		var body struct {
			VideoID string `json:"videoId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		queuedVideoID = body.VideoID
	}))

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest?input=" + url.QueryEscape("Test Song"))
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 0 {
		t.Fatalf("exitCode = %d message = %q, want 0", env.ExitCode, env.Message)
	}
	if queuedVideoID != "abc12345678" {
		t.Fatalf("queued videoId = %q, want abc12345678", queuedVideoID)
	}
	if !strings.Contains(env.Message, "Added to queue") {
		t.Fatalf("message = %q, want Added to queue", env.Message)
	}
}

func TestSongRequestHandlerSearchNoResults(t *testing.T) {
	queueCalled := false
	ts, _, _ := newTestEnv(t, ytmdSongRequestHandler(t, `{}`, emptyQueueResponse, func(r *http.Request) {
		queueCalled = true
	}))

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest?input=not-valid")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Message, "no search results") {
		t.Fatalf("message = %q, want no search results", env.Message)
	}
	if queueCalled {
		t.Fatal("upstream queue was called when search returned no results")
	}
}

func TestSongRequestHandlerBlocksBlacklistedSong(t *testing.T) {
	queueCalled := false
	ts, _, _ := newTestEnv(t, ytmdSongRequestHandler(t, blockedSearchResponse, emptyQueueResponse, func(r *http.Request) {
		queueCalled = true
	}))

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest?input=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Message, "blacklist") {
		t.Fatalf("message = %q, want blacklist rejection", env.Message)
	}
	if queueCalled {
		t.Fatal("upstream queue was called for blacklisted song")
	}
}

func TestSongRequestHandlerBlocksOverlongSong(t *testing.T) {
	queueCalled := false
	ts, _, dir := newTestEnv(t, ytmdSongRequestHandler(t, testSearchResponse, emptyQueueResponse, func(r *http.Request) {
		queueCalled = true
	}))

	store, err := config.NewStore(filepath.Join(dir, "jukeboks.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if err := store.Save(config.Config{Blacklist: []string{"Rick Astley"}, MaxDuration: 120}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest?input=abc12345678")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Message, "maxDuration") {
		t.Fatalf("message = %q, want maxDuration rejection", env.Message)
	}
	if queueCalled {
		t.Fatal("upstream queue was called for overlong song")
	}
}

func TestSongRequestHandlerBlocksDuplicate(t *testing.T) {
	queueCalled := false
	ts, _, _ := newTestEnv(t, ytmdSongRequestHandler(t, testSearchResponse, duplicateQueueResponse, func(r *http.Request) {
		queueCalled = true
	}))

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest?input=abc12345678")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Message, "already in queue") {
		t.Fatalf("message = %q, want already in queue", env.Message)
	}
	if queueCalled {
		t.Fatal("upstream queue POST was called for duplicate song")
	}
}

func TestSongRequestHandlerBlocksCurrentlyPlayingDuplicate(t *testing.T) {
	queueCalled := false
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, testSearchResponse)
		case "/api/v1/queue":
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, emptyQueueResponse)
				return
			}
			queueCalled = true
			w.WriteHeader(http.StatusNoContent)
		case "/api/v1/song":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"videoId":"abc12345678","title":"Test Song","artist":"Test Artist"}`)
		default:
			t.Errorf("unexpected upstream path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	resp, err := http.Get(ts.URL + "/cmd/jb/songrequest?input=abc12345678")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Message, "already in queue") {
		t.Fatalf("message = %q, want already in queue", env.Message)
	}
	if queueCalled {
		t.Fatal("upstream queue POST was called for currently playing duplicate")
	}
}

func TestSongRequestHandlerRequiresGET(t *testing.T) {
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	resp, err := http.Post(ts.URL+"/cmd/jb/songrequest?input=abc12345678", "application/json", nil)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	env := decodeEnvelope(t, body)
	if env.ExitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Message, "requires GET") {
		t.Fatalf("message = %q, want requires GET", env.Message)
	}
}
