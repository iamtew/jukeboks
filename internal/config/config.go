package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultBlacklistEntry = "Rick Astley"
	DefaultMaxDuration    = 600
	maxDurationCap        = 86400
)

type SeedPlaylist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TrackCount int    `json:"trackCount"`
}

type Config struct {
	Blacklist           []string       `json:"blacklist"`
	MaxDuration         int            `json:"maxDuration"`
	SeedPlaylists       []SeedPlaylist `json:"seedPlaylists,omitempty"`
	ClearQueueOnRequest bool           `json:"clearQueueOnRequest"`
}

func Default() Config {
	return Config{
		Blacklist:   []string{DefaultBlacklistEntry},
		MaxDuration: DefaultMaxDuration,
	}
}

func Ensure(path string) (Config, error) {
	cfg, err := Load(path)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	cfg = Default()
	if err := Save(path, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return Normalize(Config{Blacklist: []string{DefaultBlacklistEntry}}), nil
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return Normalize(cfg), nil
}

func Save(path string, cfg Config) error {
	cfg = Normalize(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func Normalize(cfg Config) Config {
	if cfg.Blacklist == nil {
		cfg.Blacklist = []string{DefaultBlacklistEntry}
		if cfg.MaxDuration <= 0 || cfg.MaxDuration > maxDurationCap {
			cfg.MaxDuration = DefaultMaxDuration
		}
		return cfg
	}

	normalizedBlacklist := make([]string, 0, len(cfg.Blacklist))
	seen := map[string]struct{}{}
	for _, entry := range cfg.Blacklist {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalizedBlacklist = append(normalizedBlacklist, trimmed)
	}
	if len(normalizedBlacklist) == 0 {
		cfg.Blacklist = []string{DefaultBlacklistEntry}
	} else {
		cfg.Blacklist = normalizedBlacklist
	}

	if cfg.MaxDuration <= 0 || cfg.MaxDuration > maxDurationCap {
		cfg.MaxDuration = DefaultMaxDuration
	}

	normalizedSeed := make([]SeedPlaylist, 0, len(cfg.SeedPlaylists))
	seenSeed := map[string]struct{}{}
	for _, playlist := range cfg.SeedPlaylists {
		id := strings.TrimSpace(playlist.ID)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seenSeed[key]; ok {
			continue
		}
		seenSeed[key] = struct{}{}
		name := strings.TrimSpace(playlist.Name)
		if name == "" {
			name = id
		}
		trackCount := playlist.TrackCount
		if trackCount < 0 {
			trackCount = 0
		}
		normalizedSeed = append(normalizedSeed, SeedPlaylist{
			ID:         id,
			Name:       name,
			TrackCount: trackCount,
		})
	}
	cfg.SeedPlaylists = normalizedSeed

	return cfg
}
