package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"jukeboks/internal/config"
)

func CheckContent(cfg config.Config, durationSeconds int, requireDuration bool, candidates ...string) error {
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		for _, entry := range cfg.Blacklist {
			if strings.EqualFold(trimmed, strings.TrimSpace(entry)) {
				return fmt.Errorf("request blocked by blacklist entry %q", entry)
			}
			if strings.Contains(strings.ToLower(trimmed), strings.ToLower(entry)) {
				return fmt.Errorf("request blocked by blacklist entry %q", entry)
			}
		}
	}

	if requireDuration {
		if durationSeconds <= 0 {
			return fmt.Errorf("request blocked because song duration could not be verified")
		}
		if durationSeconds > cfg.MaxDuration {
			return fmt.Errorf("request blocked because duration %d exceeds maxDuration %d", durationSeconds, cfg.MaxDuration)
		}
		return nil
	}

	if durationSeconds > cfg.MaxDuration {
		return fmt.Errorf("request blocked because duration %d exceeds maxDuration %d", durationSeconds, cfg.MaxDuration)
	}

	return nil
}

func Enforce(cfg config.Config, r *http.Request) error {
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
				textCandidates = append(textCandidates, parsed...)
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	durationSeconds := 0
	for _, key := range []string{"duration", "seconds", "length"} {
		if value := strings.TrimSpace(queryValues.Get(key)); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				durationSeconds = parsed
				break
			}
		}
	}
	if durationSeconds == 0 {
		if parsed, err := parsePolicyDurationFromBody(r); err == nil {
			durationSeconds = parsed
		}
	}

	return CheckContent(cfg, durationSeconds, false, textCandidates...)
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
