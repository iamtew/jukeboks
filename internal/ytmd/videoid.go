package ytmd

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var bareVideoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func ExtractVideoID(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty input")
	}

	if bareVideoIDPattern.MatchString(input) {
		return input, nil
	}

	candidate := input
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("invalid YouTube URL or video ID")
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")

	switch {
	case host == "youtu.be":
		id := strings.Trim(parsed.Path, "/")
		if bareVideoIDPattern.MatchString(id) {
			return id, nil
		}
	case host == "youtube.com" || host == "music.youtube.com" || host == "youtube-nocookie.com":
		if id := parsed.Query().Get("v"); bareVideoIDPattern.MatchString(id) {
			return id, nil
		}

		path := strings.Trim(parsed.Path, "/")
		segments := strings.Split(path, "/")
		if len(segments) >= 2 {
			switch segments[0] {
			case "embed", "v", "shorts", "live":
				if bareVideoIDPattern.MatchString(segments[1]) {
					return segments[1], nil
				}
			}
		}
	}

	return "", fmt.Errorf("invalid YouTube URL or video ID")
}
