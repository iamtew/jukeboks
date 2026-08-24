package config

import (
	"os"
	"sync"
	"time"
)

type Store struct {
	path  string
	mu    sync.Mutex
	cfg   Config
	mtime time.Time
}

func NewStore(path string) (*Store, error) {
	cfg, err := Ensure(path)
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, cfg: cfg}
	if info, err := os.Stat(path); err == nil {
		s.mtime = info.ModTime()
	}
	return s, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Get() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		return Config{}, err
	}
	if !info.ModTime().Equal(s.mtime) {
		cfg, err := Load(s.path)
		if err != nil {
			return Config{}, err
		}
		s.cfg = cfg
		s.mtime = info.ModTime()
	}
	return s.cfg, nil
}

func (s *Store) Save(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := Save(s.path, cfg); err != nil {
		return err
	}
	loaded, err := Load(s.path)
	if err != nil {
		return err
	}
	s.cfg = loaded
	if info, err := os.Stat(s.path); err == nil {
		s.mtime = info.ModTime()
	}
	return nil
}
