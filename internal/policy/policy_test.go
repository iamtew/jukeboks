package policy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jukeboks/internal/config"
)

func TestEnforceBlocksBlacklistedQuery(t *testing.T) {
	cfg := config.Config{Blacklist: []string{"rick astley", "never gonna give you up"}, MaxDuration: 600}
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/queue?query=Never%20Gonna%20Give%20You%20Up", nil)

	if err := Enforce(cfg, req); err == nil {
		t.Fatal("Enforce() error = nil, want blacklist rejection")
	}
}

func TestEnforceBlocksBlacklistedJSONBody(t *testing.T) {
	cfg := config.Config{Blacklist: []string{"never gonna give you up"}, MaxDuration: 600}
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/queue", strings.NewReader(`{"title":"Never Gonna Give You Up"}`))

	if err := Enforce(cfg, req); err == nil {
		t.Fatal("Enforce() error = nil, want JSON body blacklist rejection")
	}
}

func TestEnforceBlocksOverlongContent(t *testing.T) {
	cfg := config.Config{Blacklist: []string{"Rick Astley"}, MaxDuration: 600}
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/queue?duration=900", nil)

	if err := Enforce(cfg, req); err == nil {
		t.Fatal("Enforce() error = nil, want maxDuration rejection")
	}
}

func TestEnforceAllowsNormalRequest(t *testing.T) {
	cfg := config.Config{Blacklist: []string{"Rick Astley"}, MaxDuration: 600}
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/play", nil)
	if err := Enforce(cfg, req); err != nil {
		t.Fatalf("Enforce() error = %v, want nil", err)
	}
}

func TestCheckContentBlocksBlacklistedSong(t *testing.T) {
	cfg := config.Config{Blacklist: []string{"Rick Astley"}, MaxDuration: 600}
	err := CheckContent(cfg, 214, true, "Never Gonna Give You Up", "Rick Astley")
	if err == nil {
		t.Fatal("CheckContent() error = nil, want blacklist rejection")
	}
}

func TestCheckContentBlocksOverlongSong(t *testing.T) {
	cfg := config.Config{Blacklist: []string{"Rick Astley"}, MaxDuration: 120}
	err := CheckContent(cfg, 214, true, "Safe Artist", "Safe Title")
	if err == nil {
		t.Fatal("CheckContent() error = nil, want maxDuration rejection")
	}
}

func TestCheckContentRequiresDurationWhenRequested(t *testing.T) {
	cfg := config.Config{Blacklist: []string{"Rick Astley"}, MaxDuration: 600}
	err := CheckContent(cfg, 0, true, "Safe Artist", "Safe Title")
	if err == nil {
		t.Fatal("CheckContent() error = nil, want missing duration rejection")
	}
}
