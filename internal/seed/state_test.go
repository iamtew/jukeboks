package seed

import "testing"

func queueWithSelected(ids ...string) map[string]any {
	items := make([]any, 0, len(ids))
	for i, id := range ids {
		renderer := map[string]any{
			"videoId": id,
			"title":   map[string]any{"runs": []any{map[string]any{"text": id}}},
		}
		if i == 0 {
			renderer["selected"] = true
		}
		items = append(items, map[string]any{"playlistPanelVideoRenderer": renderer})
	}
	return map[string]any{"items": items}
}

func TestStateRegisterAndReconcile(t *testing.T) {
	state := NewState()
	state.RegisterEnqueue("PLtest", []string{"abc12345678", "def98765432"})
	active, count, playlistCount := state.Snapshot()
	if !active || count != 2 || playlistCount != 1 {
		t.Fatalf("snapshot = active:%v count:%d playlists:%d", active, count, playlistCount)
	}

	state.SyncFromQueue(queueWithSelected("abc12345678", "xyz00000000"), "")
	if !state.IsActive() {
		t.Fatal("expected seed mode active while a seed track is playing")
	}
}

func TestRequestInsertIndexFIFO(t *testing.T) {
	state := NewState()
	state.RegisterEnqueue("PLtest", []string{"seed1111111", "seed2222222"})

	queue := queueWithSelected("seed1111111", "seed2222222")
	if got := state.RequestInsertIndex(queue, ""); got != 1 {
		t.Fatalf("first request index = %d, want 1", got)
	}

	state.AppendRequest("req11111111")
	queue = queueWithSelected("seed1111111", "req11111111", "seed2222222")
	state.SyncFromQueue(queue, "seed1111111")
	if got := state.RequestInsertIndex(queue, "seed1111111"); got != 2 {
		t.Fatalf("second request index = %d, want 2", got)
	}

	state.AppendRequest("req22222222")
	queue = queueWithSelected("seed1111111", "req11111111", "req22222222", "seed2222222")
	state.SyncFromQueue(queue, "seed1111111")
	if got := state.RequestInsertIndex(queue, "seed1111111"); got != 3 {
		t.Fatalf("third request index = %d, want 3", got)
	}
}

func TestSyncRebuildsRequestOrderFromQueue(t *testing.T) {
	state := NewState()
	state.RegisterEnqueue("PLtest", []string{"seed1111111", "seed2222222"})
	state.AppendRequest("req22222222")
	state.AppendRequest("req11111111")

	queue := queueWithSelected("seed1111111", "req11111111", "req22222222", "seed2222222")
	state.SyncFromQueue(queue, "seed1111111")
	if state.RequestCount() != 2 {
		t.Fatalf("request count = %d, want 2", state.RequestCount())
	}
	ids := state.RequestIDs()
	found := false
	for _, id := range ids {
		if id == "req11111111" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing req11111111 after sync")
	}
	if got := state.RequestInsertIndex(queue, "seed1111111"); got != 3 {
		t.Fatalf("insert index = %d, want 3", got)
	}
}

func TestStateReconcileKeepsActiveWithoutSelectedFlag(t *testing.T) {
	state := NewState()
	state.RegisterEnqueue("PLtest", []string{"abc12345678", "def98765432"})
	state.SyncFromQueue(map[string]any{
		"items": []any{
			map[string]any{"playlistPanelVideoRenderer": map[string]any{"videoId": "abc12345678"}},
			map[string]any{"playlistPanelVideoRenderer": map[string]any{"videoId": "xyz00000000"}},
		},
	}, "abc12345678")
	if !state.IsActive() {
		t.Fatal("expected seed mode active when now playing video id matches a seed track")
	}
}

func TestStateReconcileKeepsActiveWhenUpcomingSeedTracks(t *testing.T) {
	state := NewState()
	state.RegisterEnqueue("PLtest", []string{"abc12345678", "def98765432"})
	state.SyncFromQueue(queueWithSelected("abc12345678", "def98765432"), "")
	if !state.IsActive() {
		t.Fatal("expected seed mode active while upcoming seed tracks remain")
	}
}

func TestStateReconcileIgnoresEmptyQueueSnapshot(t *testing.T) {
	state := NewState()
	state.RegisterEnqueue("PLtest", []string{"abc12345678"})
	state.SyncFromQueue(map[string]any{}, "")
	if !state.IsActive() {
		t.Fatal("expected seed mode to stay active when queue snapshot is empty")
	}
}

func TestRequestTrackingSurvivesSeedIDMismatch(t *testing.T) {
	state := NewState()
	// Browse IDs that will never appear in the live queue.
	state.RegisterEnqueue("PLtest", []string{"browse-id-1", "browse-id-2"})
	state.AppendRequest("req11111111")

	queue := queueWithSelected("live-seed-playing", "req11111111", "live-seed-next")
	state.SyncFromQueue(queue, "live-seed-playing")
	if !state.IsActive() {
		t.Fatal("expected seed mode to stay active while a request is still queued")
	}
	if state.RequestCount() != 1 {
		t.Fatalf("request count = %d, want 1", state.RequestCount())
	}
	if got := state.RequestInsertIndex(queue, "live-seed-playing"); got != 2 {
		t.Fatalf("insert index = %d, want 2", got)
	}
}

func TestClearQueueOnRequestStyleSyncKeepsRequestFIFO(t *testing.T) {
	state := NewState()
	state.RegisterEnqueue("PLtest", []string{"seed1111111", "seed2222222", "seed3333333"})
	state.AppendRequest("req11111111")

	// After clearQueueOnRequest: only current seed + request remain.
	queue := queueWithSelected("seed1111111", "req11111111")
	state.SyncFromQueue(queue, "seed1111111")
	if !state.IsActive() {
		t.Fatal("expected seed mode active with current seed + request")
	}
	if got := state.RequestInsertIndex(queue, "seed1111111"); got != 2 {
		t.Fatalf("insert index = %d, want 2", got)
	}
}

func TestStateReconcileDeactivatesWhenSeedAndRequestsAreGone(t *testing.T) {
	state := NewState()
	state.RegisterEnqueue("PLtest", []string{"abc12345678", "def98765432"})
	state.SyncFromQueue(queueWithSelected("xyz00000000", "req11111111"), "")
	if state.IsActive() {
		t.Fatal("expected seed mode inactive when no seed or tracked request remains")
	}
}
