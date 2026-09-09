package ytmd

import "testing"

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "bare id", input: "dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "bare id with whitespace", input: "  dQw4w9WgXcQ  ", want: "dQw4w9WgXcQ"},
		{name: "watch url", input: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "watch url with extra params", input: "https://youtube.com/watch?v=dQw4w9WgXcQ&t=42", want: "dQw4w9WgXcQ"},
		{name: "share url with list and feature", input: "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLrAXtmRdnEQy6nuLM&index=1&feature=share", want: "dQw4w9WgXcQ"},
		{name: "share url with si param on youtu.be", input: "https://youtu.be/dQw4w9WgXcQ?si=abc123xyz", want: "dQw4w9WgXcQ"},
		{name: "music youtube with list param", input: "https://music.youtube.com/watch?v=dQw4w9WgXcQ&list=RDAMVEdQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "youtu.be", input: "https://youtu.be/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "scheme-less youtu.be", input: "youtu.be/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "music youtube", input: "https://music.youtube.com/watch?v=dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "embed", input: "https://www.youtube.com/embed/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "shorts", input: "https://www.youtube.com/shorts/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "mobile watch", input: "https://m.youtube.com/watch?v=dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "discord brackets", input: "<https://youtu.be/dQw4w9WgXcQ>", want: "dQw4w9WgXcQ"},
		{name: "trailing punctuation", input: "https://youtu.be/dQw4w9WgXcQ.", want: "dQw4w9WgXcQ"},
		{name: "youtu.be extra path segment ignored", input: "https://youtu.be/dQw4w9WgXcQ/extra", want: "dQw4w9WgXcQ"},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid id length", input: "abc123", wantErr: true},
		{name: "playlist without video", input: "https://www.youtube.com/playlist?list=PL123", wantErr: true},
		{name: "random text", input: "not a youtube link", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractVideoID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ExtractVideoID(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractVideoID(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ExtractVideoID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLooksLikeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "https url", input: "https://open.spotify.com/track/abc", want: true},
		{name: "http url", input: "http://example.com/foo", want: true},
		{name: "www prefix", input: "www.example.com/foo", want: true},
		{name: "youtu.be scheme-less", input: "youtu.be/dQw4w9WgXcQ", want: true},
		{name: "youtube playlist", input: "https://www.youtube.com/playlist?list=PL123", want: true},
		{name: "music youtube scheme-less", input: "music.youtube.com/watch?v=dQw4w9WgXcQ", want: true},
		{name: "spotify scheme-less", input: "open.spotify.com/track/abc", want: true},
		{name: "mobile scheme-less", input: "m.youtube.com/watch?v=dQw4w9WgXcQ", want: true},
		{name: "plain song query", input: "never gonna give you up", want: false},
		{name: "artist with dots", input: "a.c. newman something", want: false},
		{name: "bare video id", input: "dQw4w9WgXcQ", want: false},
		{name: "empty", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LooksLikeURL(tt.input); got != tt.want {
				t.Fatalf("LooksLikeURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestClassifySongRequestInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantID      string
		wantURLLike bool
	}{
		{name: "bare id", input: "dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ", wantURLLike: false},
		{name: "valid url", input: "https://youtu.be/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "discord + trailing junk", input: "<https://youtu.be/dQw4w9WgXcQ>.", wantID: "dQw4w9WgXcQ", wantURLLike: true},
		{name: "spotify", input: "https://open.spotify.com/track/abc", wantID: "", wantURLLike: true},
		{name: "playlist", input: "https://www.youtube.com/playlist?list=PL123", wantID: "", wantURLLike: true},
		{name: "free text", input: "never gonna give you up", wantID: "", wantURLLike: false},
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
