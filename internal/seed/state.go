package seed

import (
	"strings"
	"sync"

	"jukeboks/internal/ytmd"
)

// State tracks seed-mode jukebox queue ownership.
//
// SeedIDs  = tracks that came from a seed playlist
// Requests = ordered FIFO list of songrequest video IDs still in the queue
//
// SyncFromQueue aligns ownership with a live YTMD queue. Empty snapshots are
// ignored. Seed mode stays active while any tracked seed OR request remains.
type State struct {
	mu          sync.Mutex
	Active      bool
	SeedIDs     map[string]struct{}
	Requests    []string
	PlaylistIDs map[string]struct{}
}

func NewState() *State {
	return &State{
		SeedIDs:     map[string]struct{}{},
		Requests:    nil,
		PlaylistIDs: map[string]struct{}{},
	}
}

func (s *State) RegisterEnqueue(playlistID string, videoIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if playlistID != "" {
		s.PlaylistIDs[strings.ToLower(strings.TrimSpace(playlistID))] = struct{}{}
	}
	for _, videoID := range videoIDs {
		normalized := normalizeID(videoID)
		if normalized == "" {
			continue
		}
		s.SeedIDs[normalized] = struct{}{}
		s.Requests = removeID(s.Requests, normalized)
	}
	if len(s.SeedIDs) > 0 {
		s.Active = true
	}
}

func (s *State) AppendRequest(videoID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := normalizeID(videoID)
	if normalized == "" {
		return
	}
	delete(s.SeedIDs, normalized)
	s.Requests = removeID(s.Requests, normalized)
	s.Requests = append(s.Requests, normalized)
	s.Active = true
}

func (s *State) Snapshot() (active bool, seedCount int, playlistCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Active, len(s.SeedIDs), len(s.PlaylistIDs)
}

func (s *State) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Requests)
}

func (s *State) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Active
}

func (s *State) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = false
	s.SeedIDs = map[string]struct{}{}
	s.Requests = nil
	s.PlaylistIDs = map[string]struct{}{}
}

func (s *State) VideoIDSet() map[string]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return copyStringSet(s.SeedIDs)
}

func (s *State) RequestIDSet() map[string]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]struct{}, len(s.Requests))
	for _, id := range s.Requests {
		out[id] = struct{}{}
	}
	return out
}

func (s *State) RequestIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.Requests))
	copy(out, s.Requests)
	return out
}

func (s *State) ClassifyVideoID(videoID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := normalizeID(videoID)
	if normalized == "" {
		return "unknown"
	}
	for _, id := range s.Requests {
		if id == normalized {
			return "request"
		}
	}
	if _, ok := s.SeedIDs[normalized]; ok {
		return "seed"
	}
	return "other"
}

// RequestInsertIndex returns where a new request should land: after the current
// track and after every already-tracked request still sitting ahead of seeds.
func (s *State) RequestInsertIndex(queuePayload any, currentVideoID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return requestInsertIndexLocked(queuePayload, currentVideoID, s.Requests)
}

func requestInsertIndexLocked(queuePayload any, currentVideoID string, requests []string) int {
	currentIndex := ytmd.ResolveCurrentQueueIndex(queuePayload, currentVideoID)
	if currentIndex < 0 {
		return -1
	}

	requestSet := make(map[string]struct{}, len(requests))
	for _, id := range requests {
		requestSet[id] = struct{}{}
	}

	items := ytmd.QueueVideoIDsByIndex(queuePayload)
	insertAfter := currentIndex
	for index := currentIndex + 1; ; index++ {
		videoID, ok := items[index]
		if !ok {
			break
		}
		if _, isRequest := requestSet[videoID]; !isRequest {
			break
		}
		insertAfter = index
	}
	return insertAfter + 1
}

func (s *State) SyncFromQueue(queuePayload any, currentVideoID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncFromQueueLocked(queuePayload, currentVideoID)
}

func (s *State) ReconcileQueue(queuePayload any, currentVideoID string) {
	s.SyncFromQueue(queuePayload, currentVideoID)
}

func (s *State) syncFromQueueLocked(queuePayload any, currentVideoID string) {
	if len(s.SeedIDs) == 0 && len(s.Requests) == 0 {
		s.Active = false
		return
	}

	items := ytmd.QueueVideoIDsByIndex(queuePayload)
	if len(items) == 0 {
		return
	}

	present := map[string]struct{}{}
	maxIndex := -1
	for index, videoID := range items {
		present[videoID] = struct{}{}
		if index > maxIndex {
			maxIndex = index
		}
	}

	currentID := normalizeID(currentVideoID)
	currentIndex := ytmd.ResolveCurrentQueueIndex(queuePayload, currentVideoID)

	// While seed mode is running, treat the currently playing non-request track
	// as a seed. This survives clearQueueOnRequest + browse/live ID mismatches.
	if s.Active && currentID != "" {
		isTrackedRequest := false
		for _, id := range s.Requests {
			if id == currentID {
				isTrackedRequest = true
				break
			}
		}
		if !isTrackedRequest {
			s.SeedIDs[currentID] = struct{}{}
		}
	}

	for videoID := range s.SeedIDs {
		if _, ok := present[videoID]; !ok {
			delete(s.SeedIDs, videoID)
		}
	}

	requestSet := make(map[string]struct{}, len(s.Requests))
	for _, id := range s.Requests {
		requestSet[id] = struct{}{}
	}
	rebuilt := make([]string, 0, len(s.Requests))
	seen := map[string]struct{}{}
	for index := 0; index <= maxIndex; index++ {
		videoID, ok := items[index]
		if !ok || videoID == "" {
			continue
		}
		if _, ok := requestSet[videoID]; !ok {
			continue
		}
		if _, dup := seen[videoID]; dup {
			continue
		}
		seen[videoID] = struct{}{}
		rebuilt = append(rebuilt, videoID)
	}
	// Keep tracked requests that are temporarily missing from a lagging snapshot.
	for _, id := range s.Requests {
		if _, ok := seen[id]; ok {
			continue
		}
		if _, ok := present[id]; ok {
			continue
		}
		rebuilt = append(rebuilt, id)
		seen[id] = struct{}{}
	}
	s.Requests = rebuilt

	seedPresent := false
	for videoID := range s.SeedIDs {
		if _, ok := present[videoID]; ok {
			seedPresent = true
			break
		}
	}
	if !seedPresent && currentIndex >= 0 {
		if videoID, ok := items[currentIndex]; ok {
			if _, isSeed := s.SeedIDs[videoID]; isSeed {
				seedPresent = true
			}
		}
	}

	requestPresent := len(s.Requests) > 0
	s.Active = seedPresent || requestPresent
	if !s.Active {
		s.SeedIDs = map[string]struct{}{}
		s.Requests = nil
		s.PlaylistIDs = map[string]struct{}{}
	}
}

func normalizeID(videoID string) string {
	return strings.ToLower(strings.TrimSpace(videoID))
}

func removeID(ids []string, target string) []string {
	if len(ids) == 0 {
		return ids
	}
	out := ids[:0]
	for _, id := range ids {
		if id == target {
			continue
		}
		out = append(out, id)
	}
	return out
}

func copyStringSet(source map[string]struct{}) map[string]struct{} {
	copySet := make(map[string]struct{}, len(source))
	for key, value := range source {
		copySet[key] = value
	}
	return copySet
}
