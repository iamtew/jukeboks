package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type responseEnvelope struct {
	ExitCode int    `json:"exitCode"`
	Message  string `json:"message,omitempty"`
	Data     any    `json:"data,omitempty"`
}

type jukeboksConfig struct {
	Blacklist   []string `json:"blacklist"`
	MaxDuration int      `json:"maxDuration"`
}

func main() {
	port := flag.String("port", "42420", "HTTP server port")
	ytmdHost := flag.String("ytmd_host", "localhost", "YTMD host")
	ytmdPort := flag.String("ytmd_port", "26538", "YTMD port")
	flag.Parse()

	target := &url.URL{Scheme: "http", Host: netJoin(*ytmdHost, *ytmdPort)}
	configPath := filepath.Join(".", "jukeboks.json")
	if _, err := ensureConfigFile(configPath); err != nil {
		log.Fatalf("failed to initialize config: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, responseEnvelope{ExitCode: 0, Message: "ok"})
	})

	mux.HandleFunc("/cmd/ytmd/", func(w http.ResponseWriter, r *http.Request) {
		proxyToYTMD(target, w, r)
	})

	mux.HandleFunc("/cmd/ytmd", func(w http.ResponseWriter, r *http.Request) {
		proxyToYTMD(target, w, r)
	})

	mux.HandleFunc("/cmd/jb/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cmd/jb/queueinfo", "/cmd/jb/queueinfo/":
			queueInfoHandler(target, w, r)
			return
		case "/cmd/jb/songinfo", "/cmd/jb/songinfo/":
			songInfoHandler(target, w, r)
			return
		}
		writeJSON(w, responseEnvelope{ExitCode: 0, Message: "custom jukeboks command scaffold"})
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg, err := loadConfig(configPath)
			if err != nil {
				writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to load config: %v", err)})
				return
			}
			writeJSON(w, responseEnvelope{ExitCode: 0, Data: cfg})
		case http.MethodPost, http.MethodPut:
			var cfg jukeboksConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to decode config: %v", err)})
				return
			}
			if err := saveConfig(configPath, cfg); err != nil {
				writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to save config: %v", err)})
				return
			}
			writeJSON(w, responseEnvelope{ExitCode: 0, Message: "config saved", Data: cfg})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join("webroot", "index.html"))
			return
		}

		if strings.HasPrefix(r.URL.Path, "/admin") || strings.HasPrefix(r.URL.Path, "/overlay") {
			http.FileServer(http.Dir("webroot")).ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join("webroot", strings.TrimPrefix(r.URL.Path, "/")))
	})

	listener, actualPort, err := listenWithFallback(*port)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	server := &http.Server{Handler: mux}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	fmt.Printf("jukeboks listening on http://localhost:%s\n", actualPort)
	if err := server.Serve(listener); err != nil && !isNormalShutdownError(err) {
		log.Fatalf("server stopped unexpectedly: %v", err)
	}
}

func isNormalShutdownError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "use of closed network connection") || strings.Contains(message, "closed")
}

func listenWithFallback(port string) (net.Listener, string, error) {
	basePort, err := strconv.Atoi(port)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port %q: %w", port, err)
	}

	for attempt := 0; attempt < 10; attempt++ {
		candidatePort := basePort + attempt
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", candidatePort))
		if err == nil {
			if candidatePort != basePort {
				fmt.Printf("port %d was busy, using %d instead\n", basePort, candidatePort)
			}
			return listener, strconv.Itoa(candidatePort), nil
		}
	}

	return nil, "", fmt.Errorf("unable to bind to port %s or any fallback ports", port)
}

func queueInfoHandler(target *url.URL, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: "queueinfo requires GET"})
		return
	}

	var songPayload any
	var queuePayload any

	songResp, err := http.Get(target.String() + "/api/v1/song")
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach song endpoint: %v", err)})
		return
	}
	defer songResp.Body.Close()
	if songResp.StatusCode >= 400 {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("song endpoint returned %d", songResp.StatusCode)})
		return
	}
	if err := json.NewDecoder(songResp.Body).Decode(&songPayload); err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to decode song payload: %v", err)})
		return
	}

	queueResp, err := http.Get(target.String() + "/api/v1/queue")
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach queue endpoint: %v", err)})
		return
	}
	defer queueResp.Body.Close()
	if queueResp.StatusCode >= 400 {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("queue endpoint returned %d", queueResp.StatusCode)})
		return
	}
	if err := json.NewDecoder(queueResp.Body).Decode(&queuePayload); err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to decode queue payload: %v", err)})
		return
	}

	response := buildQueueInfoResponse(songPayload, queuePayload)
	writeJSON(w, response)
}

func songInfoHandler(target *url.URL, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: "songinfo requires GET"})
		return
	}

	var songPayload any
	songResp, err := http.Get(target.String() + "/api/v1/song")
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to reach song endpoint: %v", err)})
		return
	}
	defer songResp.Body.Close()
	if songResp.StatusCode >= 400 {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("song endpoint returned %d", songResp.StatusCode)})
		return
	}
	if err := json.NewDecoder(songResp.Body).Decode(&songPayload); err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to decode song payload: %v", err)})
		return
	}

	response := buildSongInfoResponse(songPayload)
	writeJSON(w, response)
}

func buildSongInfoResponse(songPayload any) responseEnvelope {
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

	title := asString(songData["title"])
	artist := asString(songData["artist"])
	if title == "" {
		title = asString(songData["name"])
	}
	if artist == "" {
		artist = asString(songData["artistName"])
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
	if !isPaused {
		if state, ok := songData["state"].(string); ok {
			isPaused = strings.EqualFold(state, "paused")
		}
	}

	if title == "" && artist == "" {
		return responseEnvelope{ExitCode: 1, Message: "Song unavailable. No current track information was returned by the player.", Data: map[string]any{"reason": "no_song"}}
	}

	state := "playing"
	if isPaused {
		state = "paused"
	}

	message := fmt.Sprintf("Song: %s — %s. Playback state: %s.", title, artist, state)
	return responseEnvelope{ExitCode: 0, Message: message, Data: map[string]any{"title": title, "artist": artist, "state": state}}
}

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
	if len(entries) == 0 {
		return responseEnvelope{ExitCode: 1, Message: "Queue unavailable. No queue items were returned by the player.", Data: map[string]any{"reason": "empty_queue"}}
	}

	summary := summarizeQueueEntries(entries)
	message := fmt.Sprintf("Queue: %s, totaling %s.", summary.Display, formatMinutes(summary.TotalDurationSeconds))
	return responseEnvelope{ExitCode: 0, Message: message, Data: map[string]any{"songs": summary.Items, "totalSeconds": summary.TotalDurationSeconds, "display": summary.Display}}
}

func collectQueueEntriesForCommand(payload any) []queueEntrySummary {
	var collected []queueEntrySummary
	seen := map[string]struct{}{}
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
					key := strings.ToLower(entry.Title + "::" + entry.Artist + "::" + entry.VideoID)
					if _, ok := seen[key]; !ok {
						seen[key] = struct{}{}
						collected = append(collected, entry)
					}
				}
			}
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(payload)
	return collected
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
		displayParts = append(displayParts, entry.DisplayText)
	}
	display := strings.Join(displayParts, ", ")
	if len(items) > len(visible) {
		display = display + fmt.Sprintf(", plus %d more", len(items)-len(visible))
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

	if renderer, ok := value["playlistPanelVideoRenderer"].(map[string]any); ok {
		title = extractTextFromRunsCommand(renderer["title"])
		artist = extractTextFromRunsCommand(renderer["longBylineText"])
		videoID = asString(renderer["videoId"])
		durationSeconds = parseDurationSecondsCommand(extractTextFromRunsCommand(renderer["lengthText"]))
	} else if renderer, ok := value["videoRenderer"].(map[string]any); ok {
		title = extractTextFromRunsCommand(renderer["title"])
		artist = extractTextFromRunsCommand(renderer["longBylineText"])
		videoID = asString(renderer["videoId"])
		durationSeconds = parseDurationSecondsCommand(extractTextFromRunsCommand(renderer["lengthText"]))
	} else {
		title = extractTextFromRunsCommand(value["title"])
		artist = extractTextFromRunsCommand(value["longBylineText"])
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
		displayText = fmt.Sprintf("%s — %s", title, artist)
	}
	return queueEntrySummary{Title: title, Artist: artist, VideoID: videoID, DurationSeconds: durationSeconds, DurationLabel: formatMinutes(durationSeconds), DisplayText: displayText}
}

func isQueueEntryLikeForCommand(value map[string]any) bool {
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func proxyToYTMD(target *url.URL, w http.ResponseWriter, r *http.Request) {
	cfg, err := loadConfig(filepath.Join(".", "jukeboks.json"))
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to load config: %v", err)})
		return
	}
	if err := enforceConfigPolicy(cfg, r); err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: err.Error()})
		return
	}

	upstreamURL := *target
	upstreamURL.Path = ytmdRouteForPath(r.URL.Path)
	upstreamURL.RawQuery = r.URL.RawQuery

	method := resolveUpstreamMethod(r.Method, r.URL.Path)
	if method == http.MethodGet && r.URL.Query().Get("_method") != "" {
		method = strings.ToUpper(r.URL.Query().Get("_method"))
	}
	bodyReader, err := buildUpstreamBody(method, r)
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to build upstream request body: %v", err)})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), method, upstreamURL.String(), bodyReader)
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to build upstream request: %v", err)})
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if bodyReader != nil && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(responseEnvelope{
			ExitCode: 1,
			Message:  fmt.Sprintf("failed to reach YTMD at %s: %v", target.Host, err),
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to read YTMD response: %v", err)})
		return
	}

	if resp.StatusCode >= 400 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(responseEnvelope{
			ExitCode: 1,
			Message:  fmt.Sprintf("YTMD returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))),
		})
		return
	}

	var decoded any
	if len(bytes.TrimSpace(bodyBytes)) > 0 {
		if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
			decoded = string(bodyBytes)
		}
	}

	payload := responseEnvelope{ExitCode: 0, Data: decoded}
	if decoded == nil {
		payload.Message = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}

func enforceConfigPolicy(cfg jukeboksConfig, r *http.Request) error {
	queryValues := r.URL.Query()
	textCandidates := []string{}
	for _, key := range []string{"query", "title", "artist", "name", "url"} {
		if value := strings.TrimSpace(queryValues.Get(key)); value != "" {
			textCandidates = append(textCandidates, value)
		}
	}
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
		if len(bytes.TrimSpace(bodyBytes)) > 0 {
			textCandidates = append(textCandidates, string(bodyBytes))
			if parsed, err := parsePolicyValuesFromBody(bodyBytes); err == nil {
				for _, item := range parsed {
					textCandidates = append(textCandidates, item)
				}
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	for _, candidate := range textCandidates {
		for _, entry := range cfg.Blacklist {
			if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(entry)) {
				return fmt.Errorf("request blocked by blacklist entry %q", entry)
			}
			if strings.Contains(strings.ToLower(candidate), strings.ToLower(entry)) {
				return fmt.Errorf("request blocked by blacklist entry %q", entry)
			}
		}
	}

	for _, key := range []string{"duration", "seconds", "length"} {
		if value := strings.TrimSpace(queryValues.Get(key)); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil && parsed > cfg.MaxDuration {
				return fmt.Errorf("request blocked because duration %d exceeds maxDuration %d", parsed, cfg.MaxDuration)
			}
		}
	}

	if parsed, err := parsePolicyDurationFromBody(r); err == nil && parsed > cfg.MaxDuration {
		return fmt.Errorf("request blocked because duration %d exceeds maxDuration %d", parsed, cfg.MaxDuration)
	}

	return nil
}

func parsePolicyValuesFromBody(body []byte) ([]string, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	var values []string
	var collectStringValues func(any)
	collectStringValues = func(value any) {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				values = append(values, typed)
			}
		case []any:
			for _, item := range typed {
				collectStringValues(item)
			}
		case map[string]any:
			for _, item := range typed {
				collectStringValues(item)
			}
		}
	}
	collectStringValues(payload)
	return values, nil
}

func parsePolicyDurationFromBody(r *http.Request) (int, error) {
	if r.Body == nil {
		return 0, nil
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return 0, err
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return 0, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return 0, err
	}

	for _, key := range []string{"duration", "seconds", "length"} {
		if value, ok := payload[key]; ok {
			switch typed := value.(type) {
			case float64:
				return int(typed), nil
			case int:
				return typed, nil
			case string:
				if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
					return parsed, nil
				}
			}
		}
	}

	return 0, nil
}

func ytmdRouteForPath(path string) string {
	path = strings.TrimPrefix(path, "/cmd/ytmd")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return "/api/v1/"
	}

	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) > 1 {
		last := strings.ToLower(segments[len(segments)-1])
		if isSupportedMethod(last) {
			segments = segments[:len(segments)-1]
		}
	}

	trimmedPath := "/" + strings.Join(segments, "/")
	if trimmedPath == "/" {
		return "/api/v1/"
	}

	if strings.HasPrefix(trimmedPath, "/queue/") {
		return "/api/v1/queue/" + strings.TrimPrefix(trimmedPath, "/queue/")
	}

	return "/api/v1" + trimmedPath
}

func resolveUpstreamMethod(requestMethod, path string) string {
	commandPath := strings.TrimPrefix(path, "/cmd/ytmd")
	commandPath = strings.TrimSuffix(commandPath, "/")
	if commandPath == "" {
		return requestMethod
	}

	segments := strings.Split(strings.TrimPrefix(commandPath, "/"), "/")
	if len(segments) > 0 {
		last := strings.ToLower(segments[len(segments)-1])
		if isSupportedMethod(last) {
			method := strings.ToUpper(last)
			if method == http.MethodGet {
				return requestMethod
			}
			if method == http.MethodPost {
				return method
			}
			return method
		}
	}

	if commandPath == "/queue/{index}" || commandPath == "/queue/index" {
		if requestMethod == http.MethodGet {
			return http.MethodPatch
		}
		return requestMethod
	}

	switch commandPath {
	case "/play", "/pause", "/toggle-play", "/previous", "/next", "/seek-to", "/go-back", "/go-forward", "/toggle-mute", "/switch-repeat", "/like", "/dislike", "/volume", "/fullscreen":
		return http.MethodPost
	case "/shuffle":
		if requestMethod == http.MethodPost {
			return http.MethodPost
		}
		return http.MethodGet
	case "/queue":
		if requestMethod == http.MethodPost || requestMethod == http.MethodPatch || requestMethod == http.MethodDelete {
			return requestMethod
		}
		return http.MethodGet
	default:
		return requestMethod
	}
}

func isSupportedMethod(segment string) bool {
	switch strings.ToLower(segment) {
	case "get", "post", "put", "patch", "delete":
		return true
	default:
		return false
	}
}

func buildUpstreamBody(method string, r *http.Request) (io.Reader, error) {
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
		return r.Body, nil
	}

	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(bodyBytes)) > 0 {
			return bytes.NewReader(bodyBytes), nil
		}
	}

	queryValues := r.URL.Query()
	if len(queryValues) == 0 {
		return nil, nil
	}

	payload := make(map[string]any, len(queryValues))
	for key, values := range queryValues {
		if len(values) == 1 {
			payload[key] = coerceQueryValue(values[0])
			continue
		}
		payload[key] = values
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(bodyBytes), nil
}

func coerceQueryValue(value string) any {
	if value == "" {
		return value
	}

	if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
		return strings.EqualFold(value, "true")
	}

	if parsedInt, err := strconv.ParseInt(value, 10, 64); err == nil {
		return parsedInt
	}

	if parsedFloat, err := strconv.ParseFloat(value, 64); err == nil {
		return parsedFloat
	}

	return value
}

func ensureConfigFile(path string) (jukeboksConfig, error) {
	cfg, err := loadConfig(path)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return jukeboksConfig{}, err
	}

	defaultCfg := jukeboksConfig{Blacklist: []string{"Rick Astley"}, MaxDuration: 600}
	if err := saveConfig(path, defaultCfg); err != nil {
		return jukeboksConfig{}, err
	}
	return defaultCfg, nil
}

func loadConfig(path string) (jukeboksConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return jukeboksConfig{}, err
	}

	var cfg jukeboksConfig
	if len(bytes.TrimSpace(data)) == 0 {
		cfg = jukeboksConfig{Blacklist: []string{"Rick Astley"}}
		return cfg, nil
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return jukeboksConfig{}, err
	}
	if cfg.Blacklist == nil {
		cfg.Blacklist = []string{"Rick Astley"}
	}
	if cfg.MaxDuration == 0 {
		cfg.MaxDuration = 600
	}
	return cfg, nil
}

func saveConfig(path string, cfg jukeboksConfig) error {
	if cfg.Blacklist == nil {
		cfg.Blacklist = []string{"Rick Astley"}
	}
	if cfg.MaxDuration == 0 {
		cfg.MaxDuration = 600
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func netJoin(host, port string) string {
	return host + ":" + port
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
