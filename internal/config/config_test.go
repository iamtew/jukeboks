package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureCreatesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jukeboks.json")

	cfg, err := Ensure(path)
	if err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}

	if len(cfg.Blacklist) != 1 || cfg.Blacklist[0] != DefaultBlacklistEntry {
		t.Fatalf("default blacklist = %#v, want [%s]", cfg.Blacklist, DefaultBlacklistEntry)
	}
	if cfg.MaxDuration != DefaultMaxDuration {
		t.Fatalf("default maxDuration = %d, want %d", cfg.MaxDuration, DefaultMaxDuration)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
}

func TestSavePersistsToDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jukeboks.json")

	cfg, err := Ensure(path)
	if err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}

	cfg.Blacklist = []string{"Artist A", "Artist B"}
	cfg.MaxDuration = 900
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Blacklist) != 2 || loaded.Blacklist[0] != "Artist A" || loaded.Blacklist[1] != "Artist B" {
		t.Fatalf("loaded blacklist = %#v, want [Artist A Artist B]", loaded.Blacklist)
	}
	if loaded.MaxDuration != 900 {
		t.Fatalf("loaded maxDuration = %d, want 900", loaded.MaxDuration)
	}
}

func TestLoadNormalizesMalformedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jukeboks.json")
	if err := os.WriteFile(path, []byte(`{"blacklist":["  ","Rick Astley",""],"maxDuration":-1}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Blacklist) != 1 || loaded.Blacklist[0] != DefaultBlacklistEntry {
		t.Fatalf("loaded blacklist = %#v, want [%s]", loaded.Blacklist, DefaultBlacklistEntry)
	}
	if loaded.MaxDuration != DefaultMaxDuration {
		t.Fatalf("loaded maxDuration = %d, want %d", loaded.MaxDuration, DefaultMaxDuration)
	}
}

func TestStoreReloadsWhenFileChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jukeboks.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}

	updated := Config{Blacklist: []string{"No Rick"}, MaxDuration: 120}
	data := []byte("{\n  \"blacklist\": [\n    \"No Rick\"\n  ],\n  \"maxDuration\": 120\n}\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("Chtimes returned error: %v", err)
	}

	got, err := store.Get()
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(got.Blacklist) != 1 || got.Blacklist[0] != updated.Blacklist[0] {
		t.Fatalf("reloaded blacklist = %#v, want %#v", got.Blacklist, updated.Blacklist)
	}
	if got.MaxDuration != 120 {
		t.Fatalf("reloaded maxDuration = %d, want 120", got.MaxDuration)
	}
}

func TestStoreSaveUpdatesMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jukeboks.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}

	if err := store.Save(Config{Blacklist: []string{"Artist Z"}, MaxDuration: 30}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	got, err := store.Get()
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Blacklist[0] != "Artist Z" || got.MaxDuration != 30 {
		t.Fatalf("store Get = %#v", got)
	}
}
