package ytmd

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
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

	entry, ok := findBestSearchResult(payload, query)
	if !ok || entry.VideoID == "" {
		return "", SongLookup{}, fmt.Errorf("no matching search results for %q", query)
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

func findBestSearchResult(payload any, query string) (queueEntrySummary, bool) {
	entries := collectSearchResults(payload, 40)
	if len(entries) == 0 {
		return queueEntrySummary{}, false
	}

	bestIdx := -1
	bestScore := -1
	for i, entry := range entries {
		score, matched := queryRelevanceScore(query, entry.Title, entry.Artist)
		if !matched {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return queueEntrySummary{}, false
	}
	return entries[bestIdx], true
}

func collectSearchResults(payload any, limit int) []queueEntrySummary {
	if limit <= 0 {
		return nil
	}
	out := make([]queueEntrySummary, 0, min(limit, 16))

	var consider func(map[string]any) bool
	consider = func(value map[string]any) bool {
		entry := enrichSearchEntry(value, summarizeQueueEntry(value))
		if entry.VideoID == "" {
			return false
		}
		out = append(out, entry)
		return len(out) >= limit
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

	walk(payload)
	return out
}

func queryRelevanceScore(query, title, artist string) (score int, matched bool) {
	queryNorm := normalizeMatchText(query)
	haystack := normalizeMatchText(strings.TrimSpace(title + " " + artist))
	if queryNorm == "" || haystack == "" {
		return 0, false
	}

	if queryNorm == haystack || strings.Contains(haystack, queryNorm) {
		return 1000 + len(queryNorm), true
	}

	tokens := significantQueryTokens(queryNorm)
	if len(tokens) == 0 {
		return 0, false
	}

	matchedCount := 0
	titleNorm := normalizeMatchText(title)
	for _, token := range tokens {
		if strings.Contains(haystack, token) {
			matchedCount++
			if strings.Contains(titleNorm, token) {
				score += 15
			} else {
				score += 5
			}
		}
	}

	minRequired := (len(tokens) + 1) / 2
	if len(tokens) <= 2 {
		minRequired = len(tokens)
	}
	if matchedCount < minRequired {
		return matchedCount, false
	}

	score += matchedCount * 100
	score += (matchedCount * 100) / len(tokens)
	return score, true
}

func significantQueryTokens(normalizedQuery string) []string {
	parts := strings.Fields(normalizedQuery)
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if isMatchStopword(part) {
			continue
		}
		if utf8.RuneCountInString(part) < 2 {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func normalizeMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	lastSpace := true
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSpace = false
		default:
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func isMatchStopword(token string) bool {
	switch token {
	case "a", "an", "the", "and", "or", "of", "to", "for", "in", "on", "at", "by",
		"ft", "feat", "featuring", "with", "vs", "versus", "official", "video", "audio",
		"lyrics", "lyric", "hd", "hq", "mv":
		return true
	default:
		return false
	}
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
		if lower == "video" || lower == "song" || lower == "music" || lower == "single" || lower == "ep" || lower == "album" {
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
	if videoID := extractVideoIDFromWatchEndpoint(value["watchEndpoint"]); videoID != "" {
		return videoID
	}
	return extractVideoIDDeep(value, 0)
}

func extractVideoIDDeep(value any, depth int) string {
	if depth > 10 {
		return ""
	}
	switch typed := value.(type) {
	case map[string]any:
		if videoID := extractVideoIDFromWatchEndpoint(typed["watchEndpoint"]); videoID != "" {
			return videoID
		}
		if nested, ok := typed["playNavigationEndpoint"].(map[string]any); ok {
			if videoID := extractVideoIDDeep(nested, depth+1); videoID != "" {
				return videoID
			}
		}
		for _, nested := range typed {
			if videoID := extractVideoIDDeep(nested, depth+1); videoID != "" {
				return videoID
			}
		}
	case []any:
		for _, item := range typed {
			if videoID := extractVideoIDDeep(item, depth+1); videoID != "" {
				return videoID
			}
		}
	}
	return ""
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
