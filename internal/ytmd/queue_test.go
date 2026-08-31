package ytmd

import (
	"strings"
	"testing"
)

func rendererEntry(title, artist, length, videoID string) map[string]any {
	renderer := map[string]any{
		"title":          map[string]any{"runs": []any{map[string]any{"text": title}}},
		"longBylineText": map[string]any{"runs": []any{map[string]any{"text": artist}}},
		"lengthText":     map[string]any{"runs": []any{map[string]any{"text": length}}},
	}
	if videoID != "" {
		renderer["videoId"] = videoID
	}
	return map[string]any{"playlistPanelVideoRenderer": renderer}
}

func TestBuildQueueInfoResponseWhenPlaying(t *testing.T) {
	songPayload := map[string]any{
		"song":         map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused":     false,
		"position":     120,
		"songDuration": 240,
	}
	queuePayload := map[string]any{
		"items": []any{
			rendererEntry("Song A", "Artist A", "3:20", ""),
			rendererEntry("Song B", "Artist B", "2:10", ""),
			rendererEntry("Song C", "Artist C", "1:05", ""),
			rendererEntry("Song D", "Artist D", "2:00", ""),
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}
	if !strings.Contains(resp.Message, "Queue:") {
		t.Fatalf("message = %q, want queue prefix", resp.Message)
	}
	if !strings.Contains(resp.Message, "songs") {
		t.Fatalf("message = %q, want song count summary", resp.Message)
	}
	if !strings.Contains(resp.Message, "08:35") {
		t.Fatalf("message = %q, want total duration summary", resp.Message)
	}
	if resp.Data == nil {
		t.Fatal("expected queue summary payload in data")
	}
}

func TestBuildQueueInfoResponseWhenPaused(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused": true,
	}
	queuePayload := map[string]any{"items": []any{}}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode == 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = 0, want non-zero for paused player")
	}
	if !strings.Contains(strings.ToLower(resp.Message), "queue unavailable") {
		t.Fatalf("message = %q, want queue unavailable guidance", resp.Message)
	}
}

func TestSummarizeQueueEntryHandlesWrapperRenderer(t *testing.T) {
	entry := summarizeQueueEntry(map[string]any{
		"playlistPanelVideoWrapperRenderer": map[string]any{
			"primaryRenderer": map[string]any{
				"playlistPanelVideoRenderer": map[string]any{
					"title":          map[string]any{"runs": []any{map[string]any{"text": "Overdrive"}}},
					"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Lazerhawk"}}},
					"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "4:32"}}},
				},
			},
		},
	})

	if entry.Title != "Overdrive" {
		t.Fatalf("Title = %q, want Overdrive", entry.Title)
	}
	if entry.Artist != "Lazerhawk" {
		t.Fatalf("Artist = %q, want Lazerhawk", entry.Artist)
	}
}

func TestQueueContainsVideoID(t *testing.T) {
	payload := map[string]any{
		"items": []any{
			rendererEntry("Queued Song", "Queued Artist", "3:30", "abc12345678"),
		},
	}

	if !QueueContainsVideoID(payload, "abc12345678") {
		t.Fatal("QueueContainsVideoID() = false, want true for queued video")
	}
	if QueueContainsVideoID(payload, "xyz98765432") {
		t.Fatal("QueueContainsVideoID() = true, want false for absent video")
	}
}

func TestCollectQueueEntriesForCommandPreservesOrderedDuplicates(t *testing.T) {
	payload := map[string]any{
		"items": []any{
			map[string]any{
				"title":            "Same Song",
				"alternativeTitle": "Same Song",
				"artist":           "Artist A",
				"videoId":          "abc123",
			},
			map[string]any{
				"title":   "Same Song",
				"artist":  "Artist A",
				"videoId": "abc123",
			},
			map[string]any{
				"title":   "Other Song",
				"artist":  "Artist B",
				"videoId": "xyz789",
			},
		},
	}

	entries := collectQueueEntriesForCommand(payload)
	if len(entries) != 3 {
		t.Fatalf("collectQueueEntriesForCommand() returned %d entries, want 3", len(entries))
	}
	if entries[0].Title != "Same Song" || entries[1].Title != "Same Song" || entries[2].Title != "Other Song" {
		t.Fatalf("unexpected titles: %#v", entries)
	}
}

func TestBuildQueueInfoResponseStartsFromCurrentSong(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song B", "artist": "Artist B"},
		"isPaused": false,
	}
	queuePayload := map[string]any{
		"items": []any{
			rendererEntry("Song A", "Artist A", "2:00", ""),
			rendererEntry("Song B", "Artist B", "3:00", ""),
			rendererEntry("Song C", "Artist C", "4:00", ""),
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %#v", resp.Data)
	}
	songs, ok := dataMap["songs"].([]queueEntrySummary)
	if !ok || len(songs) != 2 {
		t.Fatalf("expected 2 remaining songs, got %#v", dataMap["songs"])
	}
	if songs[0].Title != "Song B" || songs[1].Title != "Song C" {
		t.Fatalf("queue ordering = %#v, want Song B then Song C", songs)
	}
}

func TestBuildQueueInfoResponseUsesVideoIDToStartFromCurrentSong(t *testing.T) {
	songPayload := map[string]any{
		"song": map[string]any{
			"title":   "Not The Queue Title",
			"artist":  "Different Artist",
			"videoId": "video-2",
		},
		"isPaused": false,
	}
	queuePayload := map[string]any{
		"items": []any{
			rendererEntry("Song A", "Artist A", "2:00", "video-1"),
			rendererEntry("Song B", "Artist B", "3:00", "video-2"),
			rendererEntry("Song C", "Artist C", "4:00", "video-3"),
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %#v", resp.Data)
	}
	songs, ok := dataMap["songs"].([]queueEntrySummary)
	if !ok || len(songs) != 2 {
		t.Fatalf("expected 2 remaining songs, got %#v", dataMap["songs"])
	}
	if songs[0].Title != "Song B" || songs[1].Title != "Song C" {
		t.Fatalf("queue ordering = %#v, want Song B then Song C", songs)
	}
	if totalSeconds, ok := dataMap["totalSeconds"].(int); !ok || totalSeconds != 420 {
		t.Fatalf("totalSeconds = %#v, want 420", dataMap["totalSeconds"])
	}
}

func TestBuildQueueInfoResponsePrefersVideoIDOverEarlierTitleMatch(t *testing.T) {
	songPayload := map[string]any{
		"song": map[string]any{
			"title":   "Superman",
			"artist":  "Goldfinger",
			"videoId": "video-later",
		},
		"isPaused": false,
	}
	queuePayload := map[string]any{
		"items": []any{
			rendererEntry("Superman", "Goldfinger", "3:00", "video-early"),
			rendererEntry("Other", "Artist", "2:00", "video-mid"),
			rendererEntry("Superman", "Goldfinger", "3:00", "video-later"),
			rendererEntry("After", "Artist", "1:00", "video-after"),
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %#v", resp.Data)
	}
	songs, ok := dataMap["songs"].([]queueEntrySummary)
	if !ok || len(songs) != 2 {
		t.Fatalf("expected 2 remaining songs, got %#v", dataMap["songs"])
	}
	if songs[0].VideoID != "video-later" || songs[1].Title != "After" {
		t.Fatalf("queue ordering = %#v, want video-later then After", songs)
	}
}

func TestReorderQueueEntriesDoesNotMatchArtistOnly(t *testing.T) {
	entries := []queueEntrySummary{
		{Title: "Song A", Artist: "Shared Artist", VideoID: "a"},
		{Title: "Song B", Artist: "Shared Artist", VideoID: "b"},
	}
	songData := map[string]any{
		"title":  "Different Title",
		"artist": "Shared Artist",
	}

	reordered := reorderQueueEntriesFromCurrentSong(entries, songData)
	if len(reordered) != 2 || reordered[0].Title != "Song A" {
		t.Fatalf("artist-only match should not reorder, got %#v", reordered)
	}
}

func TestBuildQueueInfoResponseKeepsOnlyArtistFromByline(t *testing.T) {
	songPayload := map[string]any{
		"song":     map[string]any{"title": "Song A", "artist": "Artist A"},
		"isPaused": false,
	}
	queuePayload := map[string]any{
		"items": []any{
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"title":          map[string]any{"runs": []any{map[string]any{"text": "Overdrive"}}},
				"longBylineText": map[string]any{"runs": []any{map[string]any{"text": "Lazerhawk"}, map[string]any{"text": "•"}, map[string]any{"text": "Redline"}, map[string]any{"text": "•"}, map[string]any{"text": "2010"}}},
				"lengthText":     map[string]any{"runs": []any{map[string]any{"text": "4:32"}}},
			}},
		},
	}

	resp := buildQueueInfoResponse(songPayload, queuePayload)
	if resp.ExitCode != 0 {
		t.Fatalf("buildQueueInfoResponse() exitCode = %d, want 0", resp.ExitCode)
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %#v", resp.Data)
	}
	songs, ok := dataMap["songs"].([]queueEntrySummary)
	if !ok || len(songs) != 1 {
		t.Fatalf("expected one queue entry, got %#v", dataMap["songs"])
	}
	if songs[0].Artist != "Lazerhawk" {
		t.Fatalf("artist = %q, want Lazerhawk", songs[0].Artist)
	}
	if strings.Contains(songs[0].DisplayText, "Redline") || strings.Contains(songs[0].DisplayText, "2010") {
		t.Fatalf("display text = %q, should not include album or year", songs[0].DisplayText)
	}
}
