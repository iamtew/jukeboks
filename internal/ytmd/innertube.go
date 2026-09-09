package ytmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// innertubeAPIKey is YouTube Music's public WEB_REMIX InnerTube client key
// (shipped in the YT Music web client; not a private credential).
const innertubeAPIKey = "AIzaSyC9XL3ZjWddXya6X74dJoCTL-WEYFDNX30"

const innertubeBrowseURL = "https://music.youtube.com/youtubei/v1/browse?alt=json&key=" + innertubeAPIKey

const innertubeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36"

func innertubeClientVersion() string {
	now := time.Now().UTC()
	return fmt.Sprintf("1.%04d%02d%02d.01.00", now.Year(), int(now.Month()), now.Day())
}

func browseIDForPlaylist(playlistID string) string {
	if strings.HasPrefix(playlistID, "VL") {
		return playlistID
	}
	return "VL" + playlistID
}

func lookupPlaylistViaInnertube(ctx context.Context, playlistID string) (PlaylistLookup, error) {
	payload, err := postInnertubeBrowse(ctx, browseIDForPlaylist(playlistID))
	if err != nil {
		return PlaylistLookup{}, err
	}
	lookup, ok := parsePlaylistPayload(playlistID, payload)
	if !ok || len(lookup.Tracks) == 0 {
		return PlaylistLookup{}, fmt.Errorf("innertube returned no tracks for playlist %q", playlistID)
	}
	return lookup, nil
}

func postInnertubeBrowse(ctx context.Context, browseID string) (any, error) {
	body := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB_REMIX",
				"clientVersion": innertubeClientVersion(),
				"hl":            "en",
				"gl":            "US",
			},
			"user": map[string]any{},
		},
		"browseId": browseID,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, innertubeBrowseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", innertubeUserAgent)
	req.Header.Set("Origin", "https://music.youtube.com")
	req.Header.Set("X-Goog-Api-Format-Version", "2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if len(bodyBytes) > 0 {
			return nil, fmt.Errorf("innertube browse returned %d: %s", resp.StatusCode, string(bodyBytes))
		}
		return nil, fmt.Errorf("innertube browse returned %d", resp.StatusCode)
	}

	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode innertube response: %w", err)
	}
	return payload, nil
}

func lookupPlaylistViaYTMDSearch(ctx context.Context, client *Client, playlistID string) (PlaylistLookup, error) {
	queries := []string{
		PlaylistURL(playlistID),
		playlistID,
	}
	var lastErr error
	for _, query := range queries {
		payload, err := client.PostJSON(ctx, "/api/v1/search", map[string]any{
			"query": query,
		})
		if err != nil {
			lastErr = err
			continue
		}
		lookup, ok := parsePlaylistPayload(playlistID, payload)
		if ok && len(lookup.Tracks) > 0 {
			return lookup, nil
		}
	}
	if lastErr != nil {
		return PlaylistLookup{}, lastErr
	}
	return PlaylistLookup{}, fmt.Errorf("search returned no tracks for playlist %q", playlistID)
}
