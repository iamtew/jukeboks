package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"jukeboks/internal/config"
	"jukeboks/internal/policy"
	"jukeboks/internal/ytmd"
)

func (s *Server) removeUpcomingSeedTracks(ctx context.Context, queuePayload any, seedVideoIDs map[string]struct{}, currentVideoID string) error {
	currentIndex := ytmd.ResolveCurrentQueueIndex(queuePayload, currentVideoID)
	indices := ytmd.QueueIndicesForVideoIDs(queuePayload, seedVideoIDs)
	if len(indices) == 0 {
		return nil
	}

	toDelete := make([]int, 0, len(indices))
	for _, index := range indices {
		if currentIndex >= 0 && index <= currentIndex {
			continue
		}
		toDelete = append(toDelete, index)
	}
	return s.deleteQueueIndices(ctx, toDelete)
}

func (s *Server) deleteQueueIndices(ctx context.Context, indices []int) error {
	if len(indices) == 0 {
		return nil
	}
	sort.Sort(sort.Reverse(sort.IntSlice(indices)))
	for _, index := range indices {
		if err := s.deleteQueueIndex(ctx, index); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) deleteQueueIndex(ctx context.Context, index int) error {
	endpoint := fmt.Sprintf("/api/v1/queue/%d", index)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.YTMD.Base.String()+endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := s.YTMD.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete queue item %d: status %d", index, resp.StatusCode)
	}
	return nil
}

func (s *Server) moveQueueItem(ctx context.Context, fromIndex, toIndex int) error {
	if fromIndex == toIndex {
		return nil
	}
	endpoint := fmt.Sprintf("/api/v1/queue/%d", fromIndex)
	body, err := json.Marshal(map[string]any{"toIndex": toIndex})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, s.YTMD.Base.String()+endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.YTMD.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to move queue item %d to %d: status %d", fromIndex, toIndex, resp.StatusCode)
	}
	return nil
}

func (s *Server) waitForQueueVideo(ctx context.Context, videoID string, attempts int, delay time.Duration) (any, int, error) {
	var last any
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return last, -1, err
		}
		queuePayload, err := s.YTMD.FetchJSONWithRetry(ctx, "/api/v1/queue", 2, delay)
		if err != nil {
			return nil, -1, err
		}
		last = queuePayload
		if index := ytmd.QueueIndexForVideoID(queuePayload, videoID, true); index >= 0 {
			return queuePayload, index, nil
		}
		if attempt < attempts-1 {
			time.Sleep(delay)
		}
	}
	return last, -1, fmt.Errorf("queued video %q not visible in YTMD queue yet", videoID)
}

// repairSeedRequestBlock moves any tracked requests that drifted (e.g. stuck at
// the bottom after a lagged insert) into a contiguous block after the current track.
func (s *Server) repairSeedRequestBlock(ctx context.Context, queuePayload any, currentVideoID string) (any, error) {
	requests := s.Seed.RequestIDs()
	if len(requests) == 0 {
		return queuePayload, nil
	}

	currentIndex := ytmd.ResolveCurrentQueueIndex(queuePayload, currentVideoID)
	if currentIndex < 0 {
		return queuePayload, nil
	}

	for offset, videoID := range requests {
		desired := currentIndex + 1 + offset
		freshQueue, err := s.YTMD.FetchJSONWithRetry(ctx, "/api/v1/queue", 3, 150*time.Millisecond)
		if err != nil {
			return queuePayload, err
		}
		queuePayload = freshQueue
		currentIndex = ytmd.ResolveCurrentQueueIndex(queuePayload, currentVideoID)
		if currentIndex < 0 {
			return queuePayload, nil
		}
		desired = currentIndex + 1 + offset

		actual := ytmd.QueueIndexForVideoID(queuePayload, videoID, true)
		if actual < 0 || actual == desired {
			continue
		}
		if err := s.moveQueueItem(ctx, actual, desired); err != nil {
			return queuePayload, err
		}
		time.Sleep(150 * time.Millisecond)
	}

	freshQueue, err := s.YTMD.FetchJSONWithRetry(ctx, "/api/v1/queue", 3, 150*time.Millisecond)
	if err != nil {
		return queuePayload, nil
	}
	return freshQueue, nil
}

func (s *Server) insertRequestDuringSeedMode(ctx context.Context, queuePayload any, songPayload any, requestVideoID string) (any, error) {
	currentVideoID := ytmd.CurrentSongVideoID(songPayload)
	s.Seed.SyncFromQueue(queuePayload, currentVideoID)

	repaired, err := s.repairSeedRequestBlock(ctx, queuePayload, currentVideoID)
	if err != nil {
		return nil, fmt.Errorf("failed to repair request block: %w", err)
	}
	queuePayload = repaired
	s.Seed.SyncFromQueue(queuePayload, currentVideoID)

	targetIndex := s.Seed.RequestInsertIndex(queuePayload, currentVideoID)
	currentIndex := ytmd.ResolveCurrentQueueIndex(queuePayload, currentVideoID)

	// Native next-up insert, then slide to the end of the request FIFO block.
	_, err = s.YTMD.PostJSON(ctx, "/api/v1/queue", map[string]any{
		"videoId":        requestVideoID,
		"insertPosition": "INSERT_AFTER_CURRENT_VIDEO",
	})
	if err != nil {
		return nil, err
	}

	freshQueue, fromIndex, err := s.waitForQueueVideo(ctx, requestVideoID, 12, 150*time.Millisecond)
	if err != nil {
		// Video was accepted but not visible yet — still track it; repair on next request.
		s.Seed.AppendRequest(requestVideoID)
		if freshQueue != nil {
			s.Seed.SyncFromQueue(freshQueue, currentVideoID)
			return freshQueue, nil
		}
		return map[string]any{}, nil
	}

	if targetIndex < 0 && currentIndex >= 0 {
		targetIndex = currentIndex + 1
	}
	if targetIndex >= 0 && fromIndex != targetIndex {
		if err := s.moveQueueItem(ctx, fromIndex, targetIndex); err != nil {
			s.Seed.AppendRequest(requestVideoID)
			return freshQueue, err
		}
		time.Sleep(150 * time.Millisecond)
		if q, idx, waitErr := s.waitForQueueVideo(ctx, requestVideoID, 8, 150*time.Millisecond); waitErr == nil {
			freshQueue = q
			if idx != targetIndex {
				_ = s.moveQueueItem(ctx, idx, targetIndex)
				if q2, qErr := s.YTMD.FetchJSONWithRetry(ctx, "/api/v1/queue", 3, 150*time.Millisecond); qErr == nil {
					freshQueue = q2
				}
			}
		}
	}

	s.Seed.AppendRequest(requestVideoID)
	s.Seed.SyncFromQueue(freshQueue, currentVideoID)
	return freshQueue, nil
}

func (s *Server) enqueueSeedTracks(ctx context.Context, cfg config.Config, lookup ytmd.PlaylistLookup) (added int, skipped int, videoIDs []string, err error) {
	queuePayload, queueErr := s.YTMD.FetchJSONWithRetry(ctx, "/api/v1/queue", 2, 250*time.Millisecond)
	queueWasEmpty := queueErr != nil || len(ytmd.QueueItemVideoIDs(queuePayload)) == 0

	for _, track := range lookup.Tracks {
		candidates := []string{track.VideoID, track.Title, track.Artist}
		if track.Title != "" && track.Artist != "" {
			candidates = append(candidates, track.Title+" "+track.Artist)
		}
		if err := policy.CheckContent(cfg, track.Duration, track.Duration > 0, candidates...); err != nil {
			skipped++
			continue
		}

		if queuePayload != nil && ytmd.QueueContainsVideoID(queuePayload, track.VideoID) {
			skipped++
			continue
		}

		_, postErr := s.YTMD.PostJSON(ctx, "/api/v1/queue", map[string]any{
			"videoId":        track.VideoID,
			"insertPosition": "INSERT_AT_END",
		})
		if postErr != nil {
			return added, skipped, videoIDs, postErr
		}
		added++
		videoIDs = append(videoIDs, track.VideoID)
	}

	if queueWasEmpty && added > 0 {
		_, _ = s.YTMD.PostJSON(ctx, "/api/v1/play", map[string]any{})
	}

	return added, skipped, videoIDs, nil
}

func (s *Server) updateSeedPlaylistInConfig(playlist config.SeedPlaylist) error {
	cfg, err := s.Config.Get()
	if err != nil {
		return err
	}

	found := false
	for index, existing := range cfg.SeedPlaylists {
		if strings.EqualFold(existing.ID, playlist.ID) {
			cfg.SeedPlaylists[index] = playlist
			found = true
			break
		}
	}
	if !found {
		cfg.SeedPlaylists = append(cfg.SeedPlaylists, playlist)
	}
	return s.Config.Save(cfg)
}

func (s *Server) removeSeedPlaylistFromConfig(playlistID string) error {
	cfg, err := s.Config.Get()
	if err != nil {
		return err
	}
	filtered := make([]config.SeedPlaylist, 0, len(cfg.SeedPlaylists))
	for _, playlist := range cfg.SeedPlaylists {
		if strings.EqualFold(playlist.ID, playlistID) {
			continue
		}
		filtered = append(filtered, playlist)
	}
	cfg.SeedPlaylists = filtered
	return s.Config.Save(cfg)
}

func (s *Server) findSavedSeedPlaylist(playlistID string) (config.SeedPlaylist, bool) {
	cfg, err := s.Config.Get()
	if err != nil {
		return config.SeedPlaylist{}, false
	}
	for _, playlist := range cfg.SeedPlaylists {
		if strings.EqualFold(playlist.ID, playlistID) {
			return playlist, true
		}
	}
	return config.SeedPlaylist{}, false
}

func seedPlaylistNameNeedsRefresh(name, id string) bool {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	if name == "" || strings.EqualFold(name, id) {
		return true
	}
	return strings.HasPrefix(name, "map[")
}

func (s *Server) refreshSeedPlaylistNames(ctx context.Context, cfg *config.Config) (bool, error) {
	updated := false
	for index, playlist := range cfg.SeedPlaylists {
		if !seedPlaylistNameNeedsRefresh(playlist.Name, playlist.ID) {
			continue
		}
		lookup, err := ytmd.LookupPlaylistByID(ctx, s.YTMD, playlist.ID)
		if err != nil || strings.TrimSpace(lookup.Name) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(lookup.Name), playlist.ID) {
			continue
		}
		cfg.SeedPlaylists[index].Name = lookup.Name
		if trackCount := len(lookup.Tracks); trackCount > 0 {
			cfg.SeedPlaylists[index].TrackCount = trackCount
		}
		updated = true
	}
	return updated, nil
}
