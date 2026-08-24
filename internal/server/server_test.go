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
	"jukeboks/internal/ytmd"
)

func newTestEnv(t *testing.T, ytmdHandler http.Handler) (*httptest.Server, *httptest.Server, string) {
	t.Helper()

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
