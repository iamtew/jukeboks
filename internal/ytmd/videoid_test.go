package ytmd

import "testing"

func TestClassifySongRequestInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantID      string
		wantURLLike bool
	}{
		{name: "bare id", input: "dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "bare id whitespace", input: "  dQw4w9WgXcQ  ", wantID: "dQw4w9WgXcQ"},
		{name: "watch url", input: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "watch url extra params", input: "https://youtube.com/watch?v=dQw4w9WgXcQ&t=42", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "youtu.be", input: "https://youtu.be/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "scheme-less youtu.be", input: "youtu.be/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "music youtube", input: "https://music.youtube.com/watch?v=dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "embed", input: "https://www.youtube.com/embed/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "shorts", input: "https://www.youtube.com/shorts/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "discord brackets", input: "<https://youtu.be/dQw4w9WgXcQ>", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "trailing punctuation", input: "https://youtu.be/dQw4w9WgXcQ.", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "spotify", input: "https://open.spotify.com/track/abc", wantURLLike: true},
		{name: "playlist", input: "https://www.youtube.com/playlist?list=PL123", wantURLLike: true},
		{name: "www prefix", input: "www.example.com/foo", wantURLLike: true},
		{name: "free text", input: "never gonna give you up"},
		{name: "artist with dots", input: "a.c. newman something"},
		{name: "empty", input: ""},
		{name: "invalid id length", input: "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotURLLike := ClassifySongRequestInput(tt.input)
			if gotID != tt.wantID || gotURLLike != tt.wantURLLike {
				t.Fatalf("ClassifySongRequestInput(%q) = (%q, %v), want (%q, %v)",
					tt.input, gotID, gotURLLike, tt.wantID, tt.wantURLLike)
			}
		})
	}
}
