package ytmd

import (
	"context"
	"fmt"
	"strings"
)

type TrackLookup struct {
	VideoID  string
	Title    string
	Artist   string
	Duration int
}

type PlaylistLookup struct {
	ID     string
	Name   string
	Tracks []TrackLookup
}

func LookupPlaylistByID(ctx context.Context, client *Client, playlistID string) (PlaylistLookup, error) {
	var attempts []string

	if lookup, err := lookupPlaylistViaInnertube(ctx, playlistID); err == nil {
		return lookup, nil
	} else if err != nil {
		attempts = append(attempts, err.Error())
	}

	if lookup, err := lookupPlaylistViaYTMDSearch(ctx, client, playlistID); err == nil {
		return lookup, nil
	} else if err != nil {
		attempts = append(attempts, err.Error())
	}

	detail := strings.Join(attempts, "; ")
	if detail == "" {
		detail = "playlist must be public or viewable in YTMD"
	}
	return PlaylistLookup{}, fmt.Errorf("no tracks found for playlist %q (%s)", playlistID, detail)
}

func parsePlaylistPayload(playlistID string, payload any) (PlaylistLookup, bool) {
	name := extractPlaylistName(payload)
	if name == "" {
		name = playlistID
	}

	entries := collectQueueEntriesForCommand(payload)
	tracks := make([]TrackLookup, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		videoID := strings.TrimSpace(entry.VideoID)
		if videoID == "" {
			continue
		}
		key := strings.ToLower(videoID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tracks = append(tracks, TrackLookup{
			VideoID:  videoID,
			Title:    entry.Title,
			Artist:   entry.Artist,
			Duration: entry.DurationSeconds,
		})
	}

	if len(tracks) == 0 {
		return PlaylistLookup{}, false
	}

	return PlaylistLookup{
		ID:     playlistID,
		Name:   name,
		Tracks: tracks,
	}, true
}

func extractPlaylistName(payload any) string {
	var found string
	var walk func(any)
	walk = func(value any) {
		if found != "" {
			return
		}
		switch typed := value.(type) {
		case map[string]any:
			if header, ok := typed["header"].(map[string]any); ok {
				if renderer, ok := header["musicResponsiveHeaderRenderer"].(map[string]any); ok {
					if title := extractTextFromRunsCommand(renderer["title"]); title != "" {
						found = title
						return
					}
				}
				if renderer, ok := header["musicDetailHeaderRenderer"].(map[string]any); ok {
					if title := extractTextFromRunsCommand(renderer["title"]); title != "" {
						found = title
						return
					}
				}
				if renderer, ok := header["musicEditablePlaylistDetailHeaderRenderer"].(map[string]any); ok {
					if title := extractTextFromRunsCommand(renderer["title"]); title != "" {
						found = title
						return
					}
				}
				if renderer, ok := header["musicImmersiveHeaderRenderer"].(map[string]any); ok {
					if title := extractTextFromRunsCommand(renderer["title"]); title != "" {
						found = title
						return
					}
				}
			}
			if header, ok := typed["musicResponsiveHeaderRenderer"].(map[string]any); ok {
				if title := extractTextFromRunsCommand(header["title"]); title != "" {
					found = title
					return
				}
			}
			if header, ok := typed["musicDetailHeaderRenderer"].(map[string]any); ok {
				if title := extractTextFromRunsCommand(header["title"]); title != "" {
					found = title
					return
				}
			}
			if shelf, ok := typed["musicPlaylistShelfRenderer"].(map[string]any); ok {
				if title := extractTextFromRunsCommand(shelf["title"]); title != "" {
					found = title
					return
				}
			}
			if microformat, ok := typed["microformatDataRenderer"].(map[string]any); ok {
				if title := extractTextFromRunsCommand(microformat["title"]); title != "" {
					found = title
					return
				}
			}
			if title := extractTextFromRunsCommand(typed["title"]); title != "" {
				if _, hasTracks := typed["contents"]; hasTracks {
					found = title
					return
				}
			}
			for _, nested := range typed {
				walk(nested)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(payload)
	return found
}

// extractAlbumBrowseID finds an album browse id (MPREb_…) linked from a playlist payload.
// OLAK5uy_ album-as-playlist browses often omit the title header but still link the album page.
func extractAlbumBrowseID(payload any) string {
	var found string
	var walk func(any)
	walk = func(value any) {
		if found != "" {
			return
		}
		switch typed := value.(type) {
		case map[string]any:
			if id, ok := typed["browseId"].(string); ok {
				id = strings.TrimSpace(id)
				if strings.HasPrefix(id, "MPREb_") {
					found = id
					return
				}
			}
			for _, nested := range typed {
				walk(nested)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(payload)
	return found
}

func QueueItemVideoIDs(payload any) []string {
	items := orderedQueueItemsArray(payload)
	if items == nil {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entry := summarizeQueueEntry(itemMap)
		if entry.VideoID != "" {
			ids = append(ids, entry.VideoID)
		}
	}
	return ids
}

func QueueIndicesForVideoIDs(payload any, videoIDs map[string]struct{}) []int {
	items := orderedQueueItemsArray(payload)
	if items == nil {
		return nil
	}
	indices := make([]int, 0)
	for index, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entry := summarizeQueueEntry(itemMap)
		videoID := strings.ToLower(strings.TrimSpace(entry.VideoID))
		if videoID == "" {
			videoID = strings.ToLower(strings.TrimSpace(extractVideoIDFromValue(itemMap)))
		}
		if videoID == "" {
			continue
		}
		if _, ok := videoIDs[videoID]; ok {
			indices = append(indices, index)
		}
	}
	return indices
}

func CurrentQueueIndex(payload any) int {
	items := orderedQueueItemsArray(payload)
	if items == nil {
		return -1
	}
	for index, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if selected, ok := itemMap["selected"].(bool); ok && selected {
			return index
		}
		if renderer := extractQueueRendererCandidate(itemMap); renderer != nil {
			if selected, ok := renderer["selected"].(bool); ok && selected {
				return index
			}
		}
	}
	return -1
}

// ResolveCurrentQueueIndex returns the index of the currently playing queue item.
// It prefers the selected flag and falls back to matching currentVideoID from /api/v1/song.
func ResolveCurrentQueueIndex(payload any, currentVideoID string) int {
	if index := CurrentQueueIndex(payload); index >= 0 {
		return index
	}
	return QueueIndexForVideoID(payload, currentVideoID, true)
}

// QueueIndexForVideoID finds a queue item index by video ID.
// When preferLast is true, the last matching row wins (useful for the now-playing track).
func QueueIndexForVideoID(payload any, videoID string, preferLast bool) int {
	videoID = strings.ToLower(strings.TrimSpace(videoID))
	if videoID == "" {
		return -1
	}
	items := orderedQueueItemsArray(payload)
	if items == nil {
		return -1
	}

	if preferLast {
		for index := len(items) - 1; index >= 0; index-- {
			itemMap, ok := items[index].(map[string]any)
			if !ok {
				continue
			}
			if strings.EqualFold(queueItemVideoID(itemMap), videoID) {
				return index
			}
		}
		return -1
	}

	for index, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.EqualFold(queueItemVideoID(itemMap), videoID) {
			return index
		}
	}
	return -1
}

func queueItemVideoID(item map[string]any) string {
	entry := summarizeQueueEntry(item)
	videoID := strings.TrimSpace(entry.VideoID)
	if videoID != "" {
		return videoID
	}
	return strings.TrimSpace(extractVideoIDFromValue(item))
}

// QueueVideoIDsByIndex maps queue indices to lowercase video IDs.
func QueueVideoIDsByIndex(payload any) map[int]string {
	result := map[int]string{}
	items := orderedQueueItemsArray(payload)
	if items == nil {
		return result
	}
	for index, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		videoID := strings.ToLower(strings.TrimSpace(queueItemVideoID(itemMap)))
		if videoID != "" {
			result[index] = videoID
		}
	}
	return result
}
