package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSeedAddPlaylistRouteNot404(t *testing.T) {
	ts, _, _ := newTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))

	resp, err := http.Post(
		ts.URL+"/api/seed/playlists",
		"application/json",
		strings.NewReader(`{"input":"https://music.youtube.com/playlist?list=PLabc1234567"}`),
	)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/seed/playlists status = 404, body = %q", body)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/seed/playlists status = %d, body = %q", resp.StatusCode, body)
	}
}
