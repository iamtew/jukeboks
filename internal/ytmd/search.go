package ytmd

import (
	"context"
	"fmt"
	"strings"
)

type SongLookup struct {
	Title    string
	Artist   string
	Duration int
}

func LookupSongByQuery(ctx context.Context, client *Client, query string) (string, SongLookup, error) {
	payload, err := client.PostJSON(ctx, "/api/v1/search", map[string]any{
		"query": query,
	})
	if err != nil {
		return "", SongLookup{}, err
	}

	entry, ok := findFirstSearchResult(payload)
	if !ok || entry.VideoID == "" {
		return "", SongLookup{}, fmt.Errorf("no search results for %q", query)
	}

	return entry.VideoID, SongLookup{
		Title:    entry.Title,
		Artist:   entry.Artist,
		Duration: entry.DurationSeconds,
	}, nil
}

func LookupSongByVideoID(ctx context.Context, client *Client, videoID string) (SongLookup, error) {
	payload, err := client.PostJSON(ctx, "/api/v1/search", map[string]any{
		"query": fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
	})
	if err != nil {
		return SongLookup{}, err
	}

	entry, ok := findQueueEntryForVideoID(payload, videoID)
	if !ok || (entry.Title == "" && entry.Artist == "") {
		return SongLookup{}, fmt.Errorf("song metadata not found for video ID %q", videoID)
	}

	return SongLookup{
		Title:    entry.Title,
		Artist:   entry.Artist,
		Duration: entry.DurationSeconds,
	}, nil
}

func findQueueEntryForVideoID(payload any, videoID string) (queueEntrySummary, bool) {
	var best queueEntrySummary
	bestScore := -1

	var consider func(map[string]any)
	consider = func(value map[string]any) {
		if !mapReferencesVideoID(value, videoID) {
			return
		}
		entry := enrichSearchEntry(value, summarizeQueueEntry(value))
		if entry.VideoID == "" {
			entry.VideoID = videoID
		}
		score := songLookupScore(entry)
		if score > bestScore {
			best = entry
			bestScore = score
		}
	}

	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if shelf, ok := typed["musicCardShelfRenderer"].(map[string]any); ok {
				consider(map[string]any{"musicCardShelfRenderer": shelf})
			}
			if item, ok := typed["musicResponsiveListItemRenderer"].(map[string]any); ok {
				consider(map[string]any{"musicResponsiveListItemRenderer": item})
			}
			if isQueueEntryLikeForCommand(typed) {
				consider(typed)
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

	return best, bestScore >= 0
}

func findFirstSearchResult(payload any) (queueEntrySummary, bool) {
	var found queueEntrySummary
	var ok bool

	var consider func(map[string]any) bool
	consider = func(value map[string]any) bool {
		entry := enrichSearchEntry(value, summarizeQueueEntry(value))
		if entry.VideoID == "" {
			return false
		}
		found = entry
		return true
	}

	var walk func(any) bool
	walk = func(value any) bool {
		switch typed := value.(type) {
		case map[string]any:
			if shelf, shelfOK := typed["musicCardShelfRenderer"].(map[string]any); shelfOK {
				if consider(map[string]any{"musicCardShelfRenderer": shelf}) {
					return true
				}
			}
			if item, itemOK := typed["musicResponsiveListItemRenderer"].(map[string]any); itemOK {
				if consider(map[string]any{"musicResponsiveListItemRenderer": item}) {
					return true
				}
			}
			for _, nested := range typed {
				if walk(nested) {
					return true
				}
			}
		case []any:
			for _, item := range typed {
				if walk(item) {
					return true
				}
			}
		}
		return false
	}

	ok = walk(payload)
	return found, ok
}

func enrichSearchEntry(value map[string]any, entry queueEntrySummary) queueEntrySummary {
	if shelf, ok := value["musicCardShelfRenderer"].(map[string]any); ok {
		if entry.Title == "" {
			entry.Title = extractTextFromRunsCommand(shelf["title"])
		}
		subtitleText := extractTextFromRunsCommand(shelf["subtitle"])
		if entry.Artist == "" {
			entry.Artist = extractArtistFromSearchSubtitle(subtitleText)
		}
		if entry.DurationSeconds == 0 {
			entry.DurationSeconds = parseDurationSecondsCommand(extractDurationTokenFromSubtitle(subtitleText))
		}
		if entry.VideoID == "" {
			entry.VideoID = extractVideoIDFromValue(shelf)
		}
	}

	if renderer := extractQueueRendererCandidate(value); renderer != nil {
		flexTitle, flexArtist := extractFlexColumnMetadata(renderer)
		if entry.Title == "" {
			entry.Title = flexTitle
		}
		if entry.Artist == "" {
			entry.Artist = flexArtist
		}
		if entry.DurationSeconds == 0 {
			entry.DurationSeconds = extractFixedColumnDuration(renderer)
		}
		if entry.VideoID == "" {
			entry.VideoID = extractVideoIDFromValue(renderer)
		}
	}

	if entry.DurationSeconds == 0 {
		if duration, ok := value["songDuration"].(float64); ok {
			entry.DurationSeconds = int(duration)
		}
	}
	if entry.DurationSeconds == 0 {
		if duration, ok := value["lengthSeconds"].(float64); ok {
			entry.DurationSeconds = int(duration)
		}
	}

	return entry
}

func extractFlexColumnMetadata(renderer map[string]any) (string, string) {
	columns, ok := renderer["flexColumns"].([]any)
	if !ok {
		return "", ""
	}

	var title string
	var artist string
	if len(columns) > 0 {
		title = extractFlexColumnText(columns[0])
	}
	if len(columns) > 1 {
		artist = normalizeBylineArtistCommand(extractFlexColumnText(columns[1]))
	}
	return title, artist
}

func extractFlexColumnText(column any) string {
	columnMap, ok := column.(map[string]any)
	if !ok {
		return ""
	}
	flex, ok := columnMap["musicResponsiveListItemFlexColumnRenderer"].(map[string]any)
	if !ok {
		return extractTextFromRunsCommand(columnMap["text"])
	}
	return extractTextFromRunsCommand(flex["text"])
}

func extractFixedColumnDuration(renderer map[string]any) int {
	columns, ok := renderer["fixedColumns"].([]any)
	if !ok {
		return 0
	}
	for _, column := range columns {
		columnMap, ok := column.(map[string]any)
		if !ok {
			continue
		}
		flex, ok := columnMap["musicResponsiveListItemFixedColumnRenderer"].(map[string]any)
		if !ok {
			continue
		}
		if duration := parseDurationSecondsCommand(extractTextFromRunsCommand(flex["text"])); duration > 0 {
			return duration
		}
	}
	return 0
}

func extractArtistFromSearchSubtitle(subtitle string) string {
	subtitle = strings.TrimSpace(subtitle)
	if subtitle == "" {
		return ""
	}

	parts := strings.Split(subtitle, "•")
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		lower := strings.ToLower(candidate)
		if lower == "video" || lower == "single" || lower == "ep" || lower == "album" {
			continue
		}
		if strings.Contains(lower, " view") {
			continue
		}
		if parseDurationSecondsCommand(candidate) > 0 {
			continue
		}
		return candidate
	}

	return normalizeBylineArtistCommand(subtitle)
}

func extractDurationTokenFromSubtitle(subtitle string) string {
	parts := strings.Split(subtitle, "•")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		if parseDurationSecondsCommand(candidate) > 0 {
			return candidate
		}
	}
	return ""
}

func extractVideoIDFromValue(value map[string]any) string {
	if videoID := asString(value["videoId"]); videoID != "" {
		return videoID
	}
	if nested, ok := value["onTap"].(map[string]any); ok {
		if videoID := extractVideoIDFromWatchEndpoint(nested["watchEndpoint"]); videoID != "" {
			return videoID
		}
	}
	if nested, ok := value["navigationEndpoint"].(map[string]any); ok {
		if videoID := extractVideoIDFromWatchEndpoint(nested["watchEndpoint"]); videoID != "" {
			return videoID
		}
	}
	return extractVideoIDFromWatchEndpoint(value["watchEndpoint"])
}

func extractVideoIDFromWatchEndpoint(value any) string {
	nested, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return asString(nested["videoId"])
}

func mapReferencesVideoID(value map[string]any, videoID string) bool {
	if strings.EqualFold(asString(value["videoId"]), videoID) {
		return true
	}
	if strings.EqualFold(asString(value["id"]), videoID) {
		return true
	}
	if strings.EqualFold(extractVideoIDFromValue(value), videoID) {
		return true
	}
	if shelf, ok := value["musicCardShelfRenderer"].(map[string]any); ok {
		if strings.EqualFold(extractVideoIDFromValue(shelf), videoID) {
			return true
		}
	}
	if item, ok := value["musicResponsiveListItemRenderer"].(map[string]any); ok {
		if strings.EqualFold(asString(item["videoId"]), videoID) {
			return true
		}
		if strings.EqualFold(extractVideoIDFromValue(item), videoID) {
			return true
		}
	}
	if nested, ok := value["watchEndpoint"].(map[string]any); ok {
		if strings.EqualFold(asString(nested["videoId"]), videoID) {
			return true
		}
	}
	if isQueueEntryLikeForCommand(value) {
		entry := summarizeQueueEntry(value)
		return strings.EqualFold(entry.VideoID, videoID)
	}
	return false
}

func songLookupScore(entry queueEntrySummary) int {
	score := 0
	if entry.Title != "" {
		score++
	}
	if entry.Artist != "" {
		score++
	}
	if entry.DurationSeconds > 0 {
		score++
	}
	return score
}
