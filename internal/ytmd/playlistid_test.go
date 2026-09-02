package ytmd

import "testing"

func TestExtractPlaylistID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "bare PL id", input: "PLabc123xyz", want: "PLabc123xyz"},
		{name: "bare OLAK id", input: "OLAK5uy_test123", want: "OLAK5uy_test123"},
		{name: "youtube playlist url", input: "https://www.youtube.com/playlist?list=PLtest123456", want: "PLtest123456"},
		{name: "music youtube album url", input: "https://music.youtube.com/playlist?list=OLAK5uy_lWZFpOGFbOSIMBTWpMuh_kkqESh9sxsuY", want: "OLAK5uy_lWZFpOGFbOSIMBTWpMuh_kkqESh9sxsuY"},
		{name: "schemeless url", input: "music.youtube.com/playlist?list=PLscheme1234", want: "PLscheme1234"},
		{name: "video url with list param ignored by videoid", input: "https://www.youtube.com/watch?v=abc12345678&list=PLmixed12345", want: "PLmixed12345"},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid", input: "not-a-playlist", wantErr: true},
		{name: "playlist without list param", input: "https://www.youtube.com/playlist", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractPlaylistID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ExtractPlaylistID(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractPlaylistID(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ExtractPlaylistID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
