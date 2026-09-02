package ytmd

import (
	"fmt"
	"strconv"
	"strings"
)

func buildQueueInfoResponse(songPayload, queuePayload any) responseEnvelope {
	songData := map[string]any{}
	if songPayloadMap, ok := songPayload.(map[string]any); ok {
		songData = songPayloadMap
	}
	if nested, ok := songPayloadMapValue(songData, "data"); ok {
		for key, value := range nested {
			songData[key] = value
		}
	}
	if nested, ok := songPayloadMapValue(songData, "song"); ok {
		for key, value := range nested {
			songData[key] = value
		}
	}

	queueData := map[string]any{}
	if queuePayloadMap, ok := queuePayload.(map[string]any); ok {
		queueData = queuePayloadMap
	}
	if nested, ok := songPayloadMapValue(queueData, "data"); ok {
		queueData = nested
	}

	isPaused := false
	if paused, ok := songData["isPaused"].(bool); ok {
		isPaused = paused
	}
	if !isPaused {
		if playing, ok := songData["isPlaying"].(bool); ok {
			isPaused = !playing
		}
	}
	if isPaused {
		return responseEnvelope{ExitCode: 1, Message: "Queue unavailable while the player is paused. Playback must be active before a queue summary can be generated.", Data: map[string]any{"reason": "player_paused"}}
	}

	entries := collectQueueEntriesForCommand(queueData)
	entries = reorderQueueEntriesFromCurrentSong(entries, songData)
	if len(entries) == 0 {
		return responseEnvelope{ExitCode: 1, Message: "Queue unavailable. No queue items were returned by the player.", Data: map[string]any{"reason": "empty_queue"}}
	}

	summary := summarizeQueueEntries(entries)
	message := fmt.Sprintf("Queue: %s. %d songs, %s total.", summary.Display, len(summary.Items), formatMinutes(summary.TotalDurationSeconds))
	return responseEnvelope{ExitCode: 0, Message: message, Data: map[string]any{"songs": summary.Items, "totalSeconds": summary.TotalDurationSeconds, "display": summary.Display, "count": len(summary.Items)}}
}

func BuildQueueInfoResponse(songPayload, queuePayload any) any {
	return buildQueueInfoResponse(songPayload, queuePayload)
}

func buildSongRequestResponse(videoID string, queuePayload any, lookup SongLookup) responseEnvelope {
	data := map[string]any{"videoId": videoID}
	title := lookup.Title
	artist := lookup.Artist

	if payloadMap, ok := queuePayload.(map[string]any); ok && len(payloadMap) > 0 {
		payloadTitle, payloadArtist := extractSongMetadata(payloadMap)
		if nested, ok := songPayloadMapValue(payloadMap, "data"); ok {
			nestedTitle, nestedArtist := extractSongMetadata(nested)
			if payloadTitle == "" {
				payloadTitle = nestedTitle
			}
			if payloadArtist == "" {
				payloadArtist = nestedArtist
			}
		}
		if nested, ok := songPayloadMapValue(payloadMap, "song"); ok {
			nestedTitle, nestedArtist := extractSongMetadata(nested)
			if payloadTitle == "" {
				payloadTitle = nestedTitle
			}
			if payloadArtist == "" {
				payloadArtist = nestedArtist
			}
		}
		if payloadTitle != "" {
			title = payloadTitle
		}
		if payloadArtist != "" {
			artist = payloadArtist
		}
	}

	if title != "" {
		data["title"] = title
	}
	if artist != "" {
		data["artist"] = artist
	}

	message := fmt.Sprintf("Added to queue: %s.", videoID)
	if title != "" && artist != "" {
		message = fmt.Sprintf("Added to queue: %s — %s.", title, artist)
	} else if title != "" {
		message = fmt.Sprintf("Added to queue: %s.", title)
	}

	return responseEnvelope{ExitCode: 0, Message: message, Data: data}
}

func extractSongMetadata(payload map[string]any) (string, string) {
	title := asString(payload["title"])
	if title == "" {
		title = asString(payload["name"])
	}
	artist := asString(payload["artist"])
	if artist == "" {
		artist = asString(payload["artistName"])
	}
	return title, artist
}

func BuildSongRequestResponse(videoID string, queuePayload any, lookup SongLookup) any {
	return buildSongRequestResponse(videoID, queuePayload, lookup)
}

func QueueContainsVideoID(payload any, videoID string) bool {
	for _, item := range orderedQueueItemsArray(payload) {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entry := summarizeQueueEntry(itemMap)
		if strings.EqualFold(entry.VideoID, videoID) {
			return true
		}
		if strings.EqualFold(extractVideoIDFromValue(itemMap), videoID) {
			return true
		}
	}
	return false
}

func CurrentSongVideoID(songPayload any) string {
	songData := map[string]any{}
	if songPayloadMap, ok := songPayload.(map[string]any); ok {
		songData = songPayloadMap
	}
	if nested, ok := songPayloadMapValue(songData, "data"); ok {
		for key, value := range nested {
			songData[key] = value
		}
	}
	if nested, ok := songPayloadMapValue(songData, "song"); ok {
		for key, value := range nested {
			songData[key] = value
		}
	}

	videoID := asString(songData["videoId"])
	if videoID == "" {
		videoID = asString(songData["video_id"])
	}
	return videoID
}

func collectQueueEntriesForCommand(payload any) []queueEntrySummary {
	if payload == nil {
		return nil
	}

	if ordered := orderedQueueItemsArray(payload); ordered != nil {
		collected := make([]queueEntrySummary, 0, len(ordered))
		for _, item := range ordered {
			typed, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if !isQueueEntryLikeForCommand(typed) {
				continue
			}
			entry := summarizeQueueEntry(typed)
			if entry.Title == "" && entry.Artist == "" && entry.VideoID == "" {
				continue
			}
			collected = append(collected, entry)
		}
		return collected
	}

	queueCandidates := []any{}
	var collectCandidates func(any)
	collectCandidates = func(value any) {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				collectCandidates(item)
			}
		case map[string]any:
			for _, key := range []string{"items", "entries", "contents", "content", "queue"} {
				if nested, ok := typed[key]; ok {
					queueCandidates = append(queueCandidates, nested)
				}
			}
			for _, item := range typed {
				collectCandidates(item)
			}
		}
	}
	collectCandidates(payload)

	if len(queueCandidates) == 0 {
		queueCandidates = append(queueCandidates, payload)
	}

	var collected []queueEntrySummary
	seen := map[string]struct{}{}
	for _, candidate := range queueCandidates {
		var walk func(any)
		walk = func(value any) {
			switch typed := value.(type) {
			case []any:
				for _, item := range typed {
					walk(item)
				}
			case map[string]any:
				if isQueueEntryLikeForCommand(typed) {
					entry := summarizeQueueEntry(typed)
					if entry.Title != "" {
						identityKeys := queueEntryIdentityKeys(typed, entry)
						if len(identityKeys) == 0 {
							return
						}
						matched := false
						for _, key := range identityKeys {
							normalizedKey := strings.ToLower(key)
							if _, ok := seen[normalizedKey]; ok {
								matched = true
								break
							}
						}
						if matched {
							return
						}
						for _, key := range identityKeys {
							normalizedKey := strings.ToLower(key)
							seen[normalizedKey] = struct{}{}
						}
						collected = append(collected, entry)
					}
				}
				for _, key := range []string{"items", "entries", "contents", "content", "queue"} {
					if nested, ok := typed[key]; ok {
						walk(nested)
					}
				}
				for _, item := range typed {
					walk(item)
				}
			}
		}
		walk(candidate)
	}
	return collected
}

func orderedQueueItemsArray(payload any) []any {
	switch typed := payload.(type) {
	case []any:
		return typed
	case map[string]any:
		if nested, ok := typed["data"].(map[string]any); ok {
			if items := orderedQueueItemsArray(nested); items != nil {
				return items
			}
		}
		for _, key := range []string{"items", "entries", "contents", "content", "queue"} {
			if items, ok := typed[key].([]any); ok {
				return items
			}
		}
	}
	return nil
}

type queueEntrySummary struct {
	Title           string
	Artist          string
	VideoID         string
	DurationSeconds int
	DurationLabel   string
	DisplayText     string
}

type queueSummary struct {
	Items                []queueEntrySummary
	Display              string
	ExtraCount           int
	TotalDurationSeconds int
}

func reorderQueueEntriesFromCurrentSong(entries []queueEntrySummary, songData map[string]any) []queueEntrySummary {
	if len(entries) <= 1 {
		return entries
	}

	currentTitle := asString(songData["title"])
	if currentTitle == "" {
		currentTitle = asString(songData["name"])
	}
	currentArtist := asString(songData["artist"])
	if currentArtist == "" {
		currentArtist = asString(songData["artistName"])
	}
	currentVideoID := asString(songData["videoId"])
	if currentVideoID == "" {
		currentVideoID = asString(songData["video_id"])
	}

	if currentTitle == "" && currentArtist == "" && currentVideoID == "" {
		return entries
	}

	currentIndex := -1
	if currentVideoID != "" {
		for idx := len(entries) - 1; idx >= 0; idx-- {
			if strings.EqualFold(entries[idx].VideoID, currentVideoID) {
				currentIndex = idx
				break
			}
		}
	}
	if currentIndex < 0 && currentTitle != "" && currentArtist != "" {
		for idx, entry := range entries {
			if strings.EqualFold(entry.Title, currentTitle) && strings.EqualFold(entry.Artist, currentArtist) {
				currentIndex = idx
				break
			}
		}
	}
	if currentIndex < 0 && currentTitle != "" && currentArtist == "" {
		for idx, entry := range entries {
			if strings.EqualFold(entry.Title, currentTitle) {
				currentIndex = idx
				break
			}
		}
	}

	if currentIndex <= 0 {
		return entries
	}

	remaining := append([]queueEntrySummary(nil), entries[currentIndex:]...)
	return remaining
}

func summarizeQueueEntries(entries []queueEntrySummary) queueSummary {
	if len(entries) == 0 {
		return queueSummary{}
	}

	items := make([]queueEntrySummary, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry)
	}

	visible := items
	if len(visible) > 3 {
		visible = visible[:3]
	}

	displayParts := make([]string, 0, len(visible))
	for _, entry := range visible {
		if entry.Artist != "" && entry.Title != "" {
			displayParts = append(displayParts, fmt.Sprintf("%s - %s", entry.Artist, entry.Title))
		} else {
			displayParts = append(displayParts, entry.DisplayText)
		}
	}
	display := strings.Join(displayParts, ", ")
	if len(items) > len(visible) {
		display = display + fmt.Sprintf(", +%d more", len(items)-len(visible))
	}

	totalSeconds := 0
	for _, entry := range items {
		totalSeconds += entry.DurationSeconds
	}
	return queueSummary{Items: items, Display: display, ExtraCount: max(0, len(items)-3), TotalDurationSeconds: totalSeconds}
}

func summarizeQueueEntry(value map[string]any) queueEntrySummary {
	var title string
	var artist string
	var videoID string
	var durationSeconds int

	renderer := extractQueueRendererCandidate(value)
	if renderer != nil {
		title = extractTextFromRunsCommand(renderer["title"])
		artist = normalizeBylineArtistCommand(renderer["longBylineText"])
		videoID = asString(renderer["videoId"])
		durationSeconds = parseDurationSecondsCommand(extractTextFromRunsCommand(renderer["lengthText"]))
	} else {
		title = extractTextFromRunsCommand(value["title"])
		artist = normalizeBylineArtistCommand(value["longBylineText"])
		videoID = asString(value["videoId"])
		durationSeconds = parseDurationSecondsCommand(extractTextFromRunsCommand(value["lengthText"]))
	}

	if title == "" {
		if titleValue, ok := value["title"].(string); ok {
			title = titleValue
		}
	}
	if artist == "" {
		if artistValue, ok := value["artist"].(string); ok {
			artist = artistValue
		}
	}
	if videoID == "" {
		videoID = asString(value["id"])
	}
	if durationSeconds == 0 {
		durationSeconds = parseDurationSecondsCommand(asString(value["duration"]))
	}

	displayText := title
	if artist != "" {
		displayText = fmt.Sprintf("%s - %s", artist, title)
	}
	entry := queueEntrySummary{Title: title, Artist: artist, VideoID: videoID, DurationSeconds: durationSeconds, DurationLabel: formatMinutes(durationSeconds), DisplayText: displayText}
	entry = enrichSearchEntry(value, entry)
	if entry.DisplayText == "" {
		if entry.Artist != "" && entry.Title != "" {
			entry.DisplayText = fmt.Sprintf("%s - %s", entry.Artist, entry.Title)
		} else {
			entry.DisplayText = entry.Title
		}
	}
	if entry.DurationLabel == "" && entry.DurationSeconds > 0 {
		entry.DurationLabel = formatMinutes(entry.DurationSeconds)
	}
	return entry
}

func extractQueueRendererCandidate(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}

	for _, key := range []string{"playlistPanelVideoRenderer", "videoRenderer", "musicResponsiveListItemRenderer", "playlistPanelRenderer"} {
		if renderer, ok := value[key].(map[string]any); ok {
			return renderer
		}
	}

	for _, key := range []string{"primaryRenderer", "renderer"} {
		if nested, ok := value[key].(map[string]any); ok {
			if renderer := extractQueueRendererCandidate(nested); renderer != nil {
				return renderer
			}
		}
	}

	if nested, ok := value["playlistPanelVideoWrapperRenderer"].(map[string]any); ok {
		if renderer := extractQueueRendererCandidate(nested); renderer != nil {
			return renderer
		}
	}

	for _, candidate := range value {
		nested, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if renderer := extractQueueRendererCandidate(nested); renderer != nil {
			return renderer
		}
	}

	return nil
}

func queueEntryIdentityKeys(value map[string]any, entry queueEntrySummary) []string {
	keys := []string{}

	titleVariants := extractQueueTitleVariants(value)
	artistVariants := extractQueueArtistVariants(value)
	if entry.Title != "" {
		titleVariants = append(titleVariants, entry.Title)
	}
	if entry.Artist != "" {
		artistVariants = append(artistVariants, entry.Artist)
	}
	titleVariants = uniqueStrings(titleVariants)
	artistVariants = uniqueStrings(artistVariants)

	for _, title := range titleVariants {
		for _, artist := range artistVariants {
			if title != "" && artist != "" {
				keys = append(keys, "title-artist:"+normalizeQueueIdentityToken(title)+":"+normalizeQueueIdentityToken(artist))
			}
		}
	}

	if len(keys) == 0 {
		if entry.VideoID != "" {
			keys = append(keys, "video:"+strings.ToLower(strings.TrimSpace(entry.VideoID)))
		}
		return uniqueStrings(keys)
	}

	if entry.VideoID != "" {
		keys = append(keys, "video:"+strings.ToLower(strings.TrimSpace(entry.VideoID)))
	}

	return uniqueStrings(keys)
}

func extractQueueTitleVariants(value map[string]any) []string {
	variants := []string{}
	for _, key := range []string{"title", "titleText", "displayTitle", "shortTitle", "songTitle", "track", "name", "alternativeTitle", "altTitle", "secondaryTitle"} {
		if text := extractTextFromRunsCommand(value[key]); text != "" {
			variants = append(variants, text)
		}
	}
	if renderer := extractQueueRendererCandidate(value); renderer != nil {
		for _, key := range []string{"title", "titleText", "displayTitle", "shortTitle", "songTitle", "track", "name", "alternativeTitle", "altTitle", "secondaryTitle"} {
			if text := extractTextFromRunsCommand(renderer[key]); text != "" {
				variants = append(variants, text)
			}
		}
	}
	return uniqueStrings(variants)
}

func extractQueueArtistVariants(value map[string]any) []string {
	variants := []string{}
	for _, key := range []string{"artist", "artistName", "author", "channel", "longBylineText", "shortBylineText", "bylineText", "authorText", "ownerText"} {
		if text := extractTextFromRunsCommand(value[key]); text != "" {
			variants = append(variants, text)
		}
	}
	if renderer := extractQueueRendererCandidate(value); renderer != nil {
		for _, key := range []string{"artist", "artistName", "author", "channel", "longBylineText", "shortBylineText", "bylineText", "authorText", "ownerText"} {
			if text := extractTextFromRunsCommand(renderer[key]); text != "" {
				variants = append(variants, text)
			}
		}
	}
	return uniqueStrings(variants)
}

func normalizeQueueIdentityToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "&", " and ")
	value = strings.ReplaceAll(value, "'", "")
	value = strings.ReplaceAll(value, "\"", "")
	value = strings.ReplaceAll(value, "  ", " ")
	return strings.TrimSpace(value)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func isQueueEntryLikeForCommand(value map[string]any) bool {
	if extractQueueRendererCandidate(value) != nil {
		return true
	}
	_, hasRenderer := value["playlistPanelVideoRenderer"]
	_, hasVideoRenderer := value["videoRenderer"]
	_, hasMusicRenderer := value["musicResponsiveListItemRenderer"]
	_, hasTitle := value["title"]
	_, hasLongByline := value["longBylineText"]
	_, hasVideoID := value["videoId"]
	_, hasID := value["id"]
	return hasRenderer || hasVideoRenderer || hasMusicRenderer || hasTitle || hasLongByline || hasVideoID || hasID
}

func extractTextFromRunsCommand(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, extractTextFromRunsCommand(item))
		}
		return strings.Join(parts, "")
	case map[string]any:
		if runs, ok := typed["runs"].([]any); ok {
			parts := make([]string, 0, len(runs))
			for _, item := range runs {
				parts = append(parts, extractTextFromRunsCommand(item))
			}
			return strings.Join(parts, "")
		}
		if text, ok := typed["text"].(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func normalizeBylineArtistCommand(value any) string {
	text := extractTextFromRunsCommand(value)
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	for _, sep := range []string{" • ", "•", " - ", " – ", " / ", " | ", " — "} {
		if strings.Contains(text, sep) {
			parts := strings.Split(text, sep)
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					return trimmed
				}
			}
		}
	}

	return text
}

func parseDurationSecondsCommand(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parts := strings.Split(value, ":")
	if len(parts) == 0 {
		return 0
	}
	if len(parts) == 2 {
		minutes, err := strconv.Atoi(parts[0])
		seconds, err2 := strconv.Atoi(parts[1])
		if err == nil && err2 == nil {
			return minutes*60 + seconds
		}
	}
	return 0
}

func formatMinutes(totalSeconds int) string {
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

func songPayloadMapValue(payload map[string]any, key string) (map[string]any, bool) {
	value, ok := payload[key]
	if !ok {
		return nil, false
	}
	mapped, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return mapped, true
}
