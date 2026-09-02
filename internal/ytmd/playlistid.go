package ytmd

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var barePlaylistIDPattern = regexp.MustCompile(`^(PL|OLAK5uy_|RD|LL|FL)[\w-]+$`)

func ExtractPlaylistID(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty input")
	}

	if barePlaylistIDPattern.MatchString(input) {
		return input, nil
	}

	candidate := input
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("invalid playlist URL or ID")
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")

	switch host {
	case "youtube.com", "music.youtube.com", "youtube-nocookie.com":
		if id := parsed.Query().Get("list"); id != "" && barePlaylistIDPattern.MatchString(id) {
			return id, nil
		}
	}

	return "", fmt.Errorf("invalid playlist URL or ID")
}

func PlaylistURL(playlistID string) string {
	return fmt.Sprintf("https://music.youtube.com/playlist?list=%s", playlistID)
}
