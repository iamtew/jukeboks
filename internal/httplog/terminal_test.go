package httplog

import (
	"strings"
	"testing"
)

func TestFormatHeader(t *testing.T) {
	UseColor = false
	line := formatHeader("http://localhost:42420", SongStatus{
		Title:  "Test Song",
		Artist: "Test Artist",
		State:  "playing",
	})

	for _, want := range []string{"Jukeboks!", "http://localhost:42420", "playing", "Test Song", "Test Artist"} {
		if !strings.Contains(line, want) {
			t.Fatalf("formatHeader() = %q, missing %q", line, want)
		}
	}
}

func TestFormatTrack(t *testing.T) {
	if got := formatTrack("Title", "Artist"); got != "Title — Artist" {
		t.Fatalf("formatTrack() = %q", got)
	}
	if got := formatTrack("", ""); got != "" {
		t.Fatalf("formatTrack() = %q, want empty", got)
	}
}

func TestStripANSI(t *testing.T) {
	plain := stripANSI("\033[1mHello\033[0m world")
	if plain != "Hello world" {
		t.Fatalf("stripANSI() = %q", plain)
	}
}

func TestTruncateANSI(t *testing.T) {
	text := "abcdefghijklmnopqrstuvwxyz"
	if got := truncateANSI(text, 10); got != "abcdefghi…" {
		t.Fatalf("truncateANSI() = %q", got)
	}
}
