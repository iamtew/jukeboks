package ytmd

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxSongRequestInputLen caps songrequest input to avoid oversized query /
// URL parsing work and pathological chat payloads.
const MaxSongRequestInputLen = 2048

var bareVideoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// knownLinkHostPrefixes are scheme-less host prefixes treated as URL attempts.
// Checked with HasPrefix on a lowercased candidate (host + optional path).
var knownLinkHostPrefixes = []string{
	"youtu.be/",
	"youtu.be?",
	"youtube.com/",
	"youtube.com?",
	"m.youtube.com/",
	"m.youtube.com?",
	"music.youtube.com/",
	"music.youtube.com?",
	"youtube-nocookie.com/",
	"youtube-nocookie.com?",
	"spotify.com/",
	"spotify.com?",
	"open.spotify.com/",
	"open.spotify.com?",
}

var trailingURLJunk = ".,!?;:)]}>\"'"

// ClassifySongRequestInput resolves songrequest input in one pass.
// When videoID is non-empty, queue that ID. When urlLike is true and videoID
// is empty, reject without searching. Otherwise treat as free-text search.
func ClassifySongRequestInput(input string) (videoID string, urlLike bool) {
	input = normalizeSongRequestInput(input)
	if input == "" {
		return "", false
	}

	if bareVideoIDPattern.MatchString(input) {
		return input, false
	}

	urlLike = hasURLShape(input)
	candidate := input
	if urlLike {
		candidate = stripTrailingURLJunk(candidate)
	}

	if id, ok := extractVideoIDFromURL(candidate); ok {
		return id, true
	}
	if urlLike {
		return "", true
	}
	return "", false
}

// LooksLikeURL reports whether input appears to be a URL (or scheme-less link
// host) rather than a free-text song query.
func LooksLikeURL(input string) bool {
	input = normalizeSongRequestInput(input)
	if input == "" || bareVideoIDPattern.MatchString(input) {
		return false
	}
	return hasURLShape(input)
}

func ExtractVideoID(input string) (string, error) {
	input = normalizeSongRequestInput(input)
	if input == "" {
		return "", fmt.Errorf("empty input")
	}

	if id, _ := ClassifySongRequestInput(input); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("invalid YouTube URL or video ID")
}

func normalizeSongRequestInput(input string) string {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "<") {
		if end := strings.IndexByte(input, '>'); end > 0 {
			inner := strings.TrimSpace(input[1:end])
			rest := input[end+1:]
			if rest == "" || stripTrailingURLJunk(rest) == "" {
				return inner
			}
		}
	}
	return input
}

func hasURLShape(input string) bool {
	lower := strings.ToLower(input)
	if strings.Contains(lower, "://") || strings.HasPrefix(lower, "www.") {
		return true
	}
	for _, prefix := range knownLinkHostPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
		hostOnly := strings.TrimSuffix(strings.TrimSuffix(prefix, "/"), "?")
		if lower == hostOnly {
			return true
		}
	}
	return false
}

func stripTrailingURLJunk(input string) string {
	for input != "" {
		r, size := utf8.DecodeLastRuneInString(input)
		if r == utf8.RuneError && size == 1 {
			break
		}
		if !strings.ContainsRune(trailingURLJunk, r) {
			break
		}
		input = input[:len(input)-size]
	}
	return input
}

func extractVideoIDFromURL(input string) (string, bool) {
	candidate := input
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", false
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")

	switch {
	case host == "youtu.be":
		id := firstPathSegment(parsed.Path)
		if bareVideoIDPattern.MatchString(id) {
			return id, true
		}
	case host == "youtube.com" || host == "music.youtube.com" || host == "youtube-nocookie.com":
		if id := parsed.Query().Get("v"); bareVideoIDPattern.MatchString(id) {
			return id, true
		}

		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(segments) >= 2 {
			switch segments[0] {
			case "embed", "v", "shorts", "live":
				if bareVideoIDPattern.MatchString(segments[1]) {
					return segments[1], true
				}
			}
		}
	}

	return "", false
}

func firstPathSegment(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return path
}
