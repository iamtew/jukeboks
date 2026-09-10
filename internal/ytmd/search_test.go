package ytmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"
)

func TestCollectSearchResultFromListItem(t *testing.T) {
	payload := map[string]any{
		"contents": map[string]any{
			"tabbedSearchResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicResponsiveListItemRenderer": map[string]any{
												"videoId": "abc12345678",
												"title": map[string]any{
													"runs": []any{
														map[string]any{"text": "Test Song"},
													},
												},
												"longBylineText": map[string]any{
													"runs": []any{
														map[string]any{"text": "Test Artist"},
													},
												},
												"lengthText": map[string]any{
													"runs": []any{
														map[string]any{"text": "3:30"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	entries := collectSearchResults(payload, 1)
	if len(entries) == 0 {
		t.Fatal("collectSearchResults() empty, want entry")
	}
	entry := entries[0]
	if entry.VideoID != "abc12345678" {
		t.Fatalf("videoId = %q, want abc12345678", entry.VideoID)
	}
	if entry.Title != "Test Song" {
		t.Fatalf("title = %q, want Test Song", entry.Title)
	}
	if entry.Artist != "Test Artist" {
		t.Fatalf("artist = %q, want Test Artist", entry.Artist)
	}
	if entry.DurationSeconds != 210 {
		t.Fatalf("duration = %d, want 210", entry.DurationSeconds)
	}
}

func TestCollectSearchResultFromMusicCardShelf(t *testing.T) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Never Gonna Give You Up"}},
					},
					"subtitle": map[string]any{
						"runs": []any{
							map[string]any{"text": "Video"},
							map[string]any{"text": " • "},
							map[string]any{"text": "Rick Astley"},
							map[string]any{"text": " • "},
							map[string]any{"text": "3:34"},
						},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "dQw4w9WgXcQ"},
					},
				},
			},
		},
	}

	entries := collectSearchResults(payload, 1)
	if len(entries) == 0 {
		t.Fatal("collectSearchResults() empty, want entry")
	}
	entry := entries[0]
	if entry.VideoID != "dQw4w9WgXcQ" {
		t.Fatalf("videoId = %q, want dQw4w9WgXcQ", entry.VideoID)
	}
	if entry.Title != "Never Gonna Give You Up" {
		t.Fatalf("title = %q", entry.Title)
	}
	if entry.Artist != "Rick Astley" {
		t.Fatalf("artist = %q", entry.Artist)
	}
	if entry.DurationSeconds != 214 {
		t.Fatalf("duration = %d, want 214", entry.DurationSeconds)
	}
}

func TestCollectSearchResultFromMusicCardShelfSongType(t *testing.T) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Never Gonna Give You Up"}},
					},
					"subtitle": map[string]any{
						"runs": []any{
							map[string]any{"text": "Song"},
							map[string]any{"text": " • "},
							map[string]any{"text": "Rick Astley"},
							map[string]any{"text": " • "},
							map[string]any{"text": "3:34"},
						},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "dQw4w9WgXcQ"},
					},
				},
			},
		},
	}

	entries := collectSearchResults(payload, 1)
	if len(entries) == 0 {
		t.Fatal("collectSearchResults() empty, want entry")
	}
	entry := entries[0]
	if entry.VideoID != "dQw4w9WgXcQ" {
		t.Fatalf("videoId = %q, want dQw4w9WgXcQ", entry.VideoID)
	}
	if entry.Title != "Never Gonna Give You Up" {
		t.Fatalf("title = %q", entry.Title)
	}
	if entry.Artist != "Rick Astley" {
		t.Fatalf("artist = %q, want Rick Astley", entry.Artist)
	}
	if entry.DurationSeconds != 214 {
		t.Fatalf("duration = %d, want 214", entry.DurationSeconds)
	}
}

func TestExtractArtistFromSearchSubtitle(t *testing.T) {
	tests := []struct {
		name     string
		subtitle string
		want     string
	}{
		{
			name:     "video prefix",
			subtitle: "Video • Rick Astley • 3:34",
			want:     "Rick Astley",
		},
		{
			name:     "song prefix",
			subtitle: "Song • Rick Astley • 3:34",
			want:     "Rick Astley",
		},
		{
			name:     "music prefix",
			subtitle: "Music • Rick Astley • 3:34",
			want:     "Rick Astley",
		},
		{
			name:     "view count segment skipped",
			subtitle: "Video • 1.2M views • Rick Astley • 3:34",
			want:     "Rick Astley",
		},
		{
			name:     "bare artist",
			subtitle: "Rick Astley",
			want:     "Rick Astley",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractArtistFromSearchSubtitle(tt.subtitle)
			if got != tt.want {
				t.Fatalf("extractArtistFromSearchSubtitle(%q) = %q, want %q", tt.subtitle, got, tt.want)
			}
		})
	}
}

func TestCollectSearchResultReturnsFirstMatch(t *testing.T) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "First Song"}},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "first1234567"},
					},
				},
			},
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Second Song"}},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "second123456"},
					},
				},
			},
		},
	}

	entries := collectSearchResults(payload, 1)
	if len(entries) == 0 {
		t.Fatal("collectSearchResults() empty, want entry")
	}
	entry := entries[0]
	if entry.VideoID != "first1234567" {
		t.Fatalf("videoId = %q, want first1234567", entry.VideoID)
	}
}

func TestFindBestSearchResultSkipsUnrelatedTopHit(t *testing.T) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Totally Unrelated Mix"}},
					},
					"subtitle": map[string]any{
						"runs": []any{map[string]any{"text": "Random DJ"}},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "random123456"},
					},
				},
			},
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Never Gonna Give You Up"}},
					},
					"subtitle": map[string]any{
						"runs": []any{
							map[string]any{"text": "Song"},
							map[string]any{"text": " • "},
							map[string]any{"text": "Rick Astley"},
						},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "dQw4w9WgXcQ"},
					},
				},
			},
		},
	}

	entry, ok := findBestSearchResult(payload, "never gonna give you up rick astley")
	if !ok {
		t.Fatal("findBestSearchResult() ok = false, want true")
	}
	if entry.VideoID != "dQw4w9WgXcQ" {
		t.Fatalf("videoId = %q, want dQw4w9WgXcQ", entry.VideoID)
	}
}

func TestFindBestSearchResultPrefersKnownDuration(t *testing.T) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Trying To Find A Balance"}},
					},
					"subtitle": map[string]any{
						"runs": []any{
							map[string]any{"text": "Song"},
							map[string]any{"text": " • "},
							map[string]any{"text": "Atmosphere"},
							map[string]any{"text": " • "},
							map[string]any{"text": "4:18"},
						},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "ttKzEo8KPjE"},
					},
				},
			},
			map[string]any{
				"musicResponsiveListItemRenderer": map[string]any{
					"videoId": "Et_E4M4JgIU",
					"flexColumns": []any{
						map[string]any{
							"musicResponsiveListItemFlexColumnRenderer": map[string]any{
								"text": map[string]any{
									"runs": []any{map[string]any{"text": "Atmosphere - Trying to Find a Balance"}},
								},
							},
						},
						map[string]any{
							"musicResponsiveListItemFlexColumnRenderer": map[string]any{
								"text": map[string]any{
									"runs": []any{map[string]any{"text": "Video • AspiringPsychopath"}},
								},
							},
						},
					},
				},
			},
		},
	}

	entry, ok := findBestSearchResult(payload, "Atmosphere Trying To Find A Balance")
	if !ok {
		t.Fatal("findBestSearchResult() ok = false, want true")
	}
	if entry.VideoID != "ttKzEo8KPjE" {
		t.Fatalf("videoId = %q, want ttKzEo8KPjE (duration-known shelf hit)", entry.VideoID)
	}
	if entry.DurationSeconds != 258 {
		t.Fatalf("duration = %d, want 258", entry.DurationSeconds)
	}
}

func TestFindBestSearchResultRejectsNoMatch(t *testing.T) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Completely Different Track"}},
					},
					"subtitle": map[string]any{
						"runs": []any{map[string]any{"text": "Other Artist"}},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "other1234567"},
					},
				},
			},
		},
	}

	_, ok := findBestSearchResult(payload, "never gonna give you up")
	if ok {
		t.Fatal("findBestSearchResult() ok = true, want false")
	}
}

func TestQueryRelevanceScore(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		title   string
		artist  string
		matched bool
	}{
		{name: "exact title", query: "never gonna give you up", title: "Never Gonna Give You Up", artist: "Rick Astley", matched: true},
		{name: "artist only query", query: "rick astley", title: "Never Gonna Give You Up", artist: "Rick Astley", matched: true},
		{name: "unrelated", query: "enter sandman", title: "Bohemian Rhapsody", artist: "Queen", matched: false},
		{name: "partial weak", query: "never gonna give you up", title: "Up", artist: "Someone", matched: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, matched := queryRelevanceScore(tt.query, tt.title, tt.artist)
			if matched != tt.matched {
				t.Fatalf("queryRelevanceScore(%q, %q, %q) matched = %v, want %v",
					tt.query, tt.title, tt.artist, matched, tt.matched)
			}
		})
	}
}

func TestCollectSearchResultEmpty(t *testing.T) {
	entries := collectSearchResults(map[string]any{}, 1)
	if len(entries) != 0 {
		t.Fatal("collectSearchResults() nonempty, want empty")
	}
}

func TestFindQueueEntryForVideoIDFromSearchShape(t *testing.T) {
	payload := map[string]any{
		"contents": map[string]any{
			"tabbedSearchResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicResponsiveListItemRenderer": map[string]any{
												"videoId": "abc12345678",
												"title": map[string]any{
													"runs": []any{
														map[string]any{"text": "Test Song"},
													},
												},
												"longBylineText": map[string]any{
													"runs": []any{
														map[string]any{"text": "Test Artist"},
													},
												},
												"lengthText": map[string]any{
													"runs": []any{
														map[string]any{"text": "3:30"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	entry, ok := findQueueEntryForVideoID(payload, "abc12345678")
	if !ok {
		t.Fatal("findQueueEntryForVideoID() ok = false, want true")
	}
	if entry.Title != "Test Song" {
		t.Fatalf("title = %q, want Test Song", entry.Title)
	}
	if entry.Artist != "Test Artist" {
		t.Fatalf("artist = %q, want Test Artist", entry.Artist)
	}
	if entry.DurationSeconds != 210 {
		t.Fatalf("duration = %d, want 210", entry.DurationSeconds)
	}
}

func TestFindQueueEntryForVideoIDFromMusicCardShelf(t *testing.T) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"musicCardShelfRenderer": map[string]any{
					"title": map[string]any{
						"runs": []any{map[string]any{"text": "Never Gonna Give You Up"}},
					},
					"subtitle": map[string]any{
						"runs": []any{
							map[string]any{"text": "Video"},
							map[string]any{"text": " • "},
							map[string]any{"text": "Rick Astley"},
							map[string]any{"text": " • "},
							map[string]any{"text": "3:34"},
						},
					},
					"onTap": map[string]any{
						"watchEndpoint": map[string]any{"videoId": "dQw4w9WgXcQ"},
					},
				},
			},
		},
	}

	entry, ok := findQueueEntryForVideoID(payload, "dQw4w9WgXcQ")
	if !ok {
		t.Fatal("findQueueEntryForVideoID() ok = false, want true")
	}
	if entry.Title != "Never Gonna Give You Up" {
		t.Fatalf("title = %q", entry.Title)
	}
	if entry.Artist != "Rick Astley" {
		t.Fatalf("artist = %q", entry.Artist)
	}
	if entry.DurationSeconds != 214 {
		t.Fatalf("duration = %d, want 214", entry.DurationSeconds)
	}
}

func TestLookupSongByVideoIDFromFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Skip("search fixture unavailable")
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	entry, ok := findQueueEntryForVideoID(payload, "dQw4w9WgXcQ")
	if !ok {
		t.Fatal("findQueueEntryForVideoID() ok = false, want true")
	}
	if entry.Title == "" || entry.Artist == "" {
		t.Fatalf("entry = %+v, want title and artist", entry)
	}
	if entry.DurationSeconds <= 0 {
		t.Fatalf("duration = %d, want > 0", entry.DurationSeconds)
	}
}

func TestLookupSongByQueryIntegration(t *testing.T) {
	if _, err := http.Get("http://localhost:26538/api/v1/song"); err != nil {
		t.Skipf("YTMD not available: %v", err)
	}

	target, err := url.Parse("http://localhost:26538")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	client := NewClient(target)
	videoID, lookup, err := LookupSongByQuery(context.Background(), client, "never gonna give you up")
	if err != nil {
		t.Fatalf("LookupSongByQuery() error = %v", err)
	}
	if videoID == "" {
		t.Fatal("expected video ID from live YTMD search")
	}
	if lookup.Title == "" {
		t.Fatal("expected title from live YTMD search")
	}
	if lookup.Artist == "" {
		t.Fatal("expected artist from live YTMD search")
	}
	if lookup.Duration <= 0 {
		t.Fatalf("duration = %d, want > 0", lookup.Duration)
	}
}

func TestLookupSongByVideoIDIntegration(t *testing.T) {
	if _, err := http.Get("http://localhost:26538/api/v1/song"); err != nil {
		t.Skipf("YTMD not available: %v", err)
	}

	target, err := url.Parse("http://localhost:26538")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	client := NewClient(target)
	lookup, err := LookupSongByVideoID(context.Background(), client, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("LookupSongByVideoID() error = %v", err)
	}
	if lookup.Title == "" {
		t.Fatal("expected title from live YTMD search")
	}
	if lookup.Artist == "" {
		t.Fatal("expected artist from live YTMD search")
	}
	if lookup.Duration <= 0 {
		t.Fatalf("duration = %d, want > 0", lookup.Duration)
	}
}
