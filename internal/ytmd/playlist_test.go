package ytmd

import "testing"

func TestParsePlaylistPayloadFromSearchFixture(t *testing.T) {
	payload := map[string]any{
		"header": map[string]any{
			"musicDetailHeaderRenderer": map[string]any{
				"title": map[string]any{
					"runs": []any{map[string]any{"text": "Test Seed Playlist"}},
				},
			},
		},
		"contents": []any{
			map[string]any{
				"musicResponsiveListItemRenderer": map[string]any{
					"videoId": "abc12345678",
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Track One"}},
					},
					"longBylineText": map[string]any{
						"runs": []any{map[string]any{"text": "Artist One"}},
					},
					"lengthText": map[string]any{
						"runs": []any{map[string]any{"text": "3:30"}},
					},
				},
			},
			map[string]any{
				"musicResponsiveListItemRenderer": map[string]any{
					"videoId": "def98765432",
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Track Two"}},
					},
					"longBylineText": map[string]any{
						"runs": []any{map[string]any{"text": "Artist Two"}},
					},
					"lengthText": map[string]any{
						"runs": []any{map[string]any{"text": "4:00"}},
					},
				},
			},
		},
	}

	lookup, ok := parsePlaylistPayload("PLtest123456", payload)
	if !ok {
		t.Fatal("parsePlaylistPayload() ok = false, want true")
	}
	if lookup.Name != "Test Seed Playlist" {
		t.Fatalf("name = %q, want Test Seed Playlist", lookup.Name)
	}
	if len(lookup.Tracks) != 2 {
		t.Fatalf("track count = %d, want 2", len(lookup.Tracks))
	}
	if lookup.Tracks[0].VideoID != "abc12345678" {
		t.Fatalf("first videoId = %q", lookup.Tracks[0].VideoID)
	}
}

func TestExtractPlaylistNameIgnoresTrackTitles(t *testing.T) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"musicResponsiveListItemRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Track One"}},
					},
				},
			},
		},
		"header": map[string]any{
			"musicDetailHeaderRenderer": map[string]any{
				"title": map[string]any{
					"runs": []any{map[string]any{"text": "Hiphop | Sesh Sofa"}},
				},
			},
		},
	}

	name := extractPlaylistName(payload)
	if name != "Hiphop | Sesh Sofa" {
		t.Fatalf("name = %q, want Hiphop | Sesh Sofa", name)
	}
}

func TestQueueIndicesForVideoIDs(t *testing.T) {
	payload := map[string]any{
		"items": []any{
			map[string]any{
				"playlistPanelVideoRenderer": map[string]any{
					"videoId": "aaa11111111",
					"title":   map[string]any{"runs": []any{map[string]any{"text": "A"}}},
				},
			},
			map[string]any{
				"playlistPanelVideoRenderer": map[string]any{
					"videoId":  "bbb22222222",
					"selected": true,
					"title":    map[string]any{"runs": []any{map[string]any{"text": "B"}}},
				},
			},
			map[string]any{
				"playlistPanelVideoRenderer": map[string]any{
					"videoId": "ccc33333333",
					"title":   map[string]any{"runs": []any{map[string]any{"text": "C"}}},
				},
			},
		},
	}

	target := map[string]struct{}{
		"aaa11111111": {},
		"ccc33333333": {},
	}
	indices := QueueIndicesForVideoIDs(payload, target)
	if len(indices) != 2 || indices[0] != 0 || indices[1] != 2 {
		t.Fatalf("indices = %v, want [0 2]", indices)
	}
}
