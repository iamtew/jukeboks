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
