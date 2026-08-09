package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestYTMDRouteMapping(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		expected string
	}{
		{name: "play", path: "/cmd/ytmd/play", expected: "/api/v1/play"},
		{name: "song", path: "/cmd/ytmd/song", expected: "/api/v1/song"},
		{name: "queue next", path: "/cmd/ytmd/queue/next", expected: "/api/v1/queue/next"},
		{name: "root", path: "/cmd/ytmd/", expected: "/api/v1/"},
		{name: "root without slash", path: "/cmd/ytmd", expected: "/api/v1/"},
		{name: "queue with method suffix", path: "/cmd/ytmd/queue/post", expected: "/api/v1/queue"},
		{name: "search with method suffix", path: "/cmd/ytmd/search/post", expected: "/api/v1/search"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ytmdRouteForPath(tc.path); got != tc.expected {
				t.Fatalf("ytmdRouteForPath(%q) = %q, want %q", tc.path, got, tc.expected)
			}
		})
	}
}

func TestMethodSuffixResolution(t *testing.T) {
	cases := []struct {
		name          string
		path          string
		requestMethod string
		expected      string
	}{
		{name: "queue post suffix", path: "/cmd/ytmd/queue/post", requestMethod: "GET", expected: "POST"},
		{name: "queue patch suffix", path: "/cmd/ytmd/queue/patch", requestMethod: "GET", expected: "PATCH"},
		{name: "queue delete suffix", path: "/cmd/ytmd/queue/delete", requestMethod: "GET", expected: "DELETE"},
		{name: "search post suffix", path: "/cmd/ytmd/search/post", requestMethod: "GET", expected: "POST"},
		{name: "plain play route", path: "/cmd/ytmd/play", requestMethod: "GET", expected: "POST"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveUpstreamMethod(tc.requestMethod, tc.path); got != tc.expected {
				t.Fatalf("resolveUpstreamMethod(%q, %q) = %q, want %q", tc.requestMethod, tc.path, got, tc.expected)
			}
		})
	}
}

func TestBuildUpstreamBodyUsesRequestBodyForPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/seek-to/post?seconds=42", bytes.NewBufferString(`{"seconds": 7}`))
	bodyReader, err := buildUpstreamBody(http.MethodPost, req)
	if err != nil {
		t.Fatalf("buildUpstreamBody returned error: %v", err)
	}

	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}

	if string(bodyBytes) != `{"seconds": 7}` {
		t.Fatalf("buildUpstreamBody() = %q, want JSON body", string(bodyBytes))
	}
}

func TestBuildUpstreamBodyFallsBackToQueryForPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cmd/ytmd/seek-to/post?seconds=42", nil)
	bodyReader, err := buildUpstreamBody(http.MethodPost, req)
	if err != nil {
		t.Fatalf("buildUpstreamBody returned error: %v", err)
	}

	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}

	if string(bodyBytes) != `{"seconds":42}` {
		t.Fatalf("buildUpstreamBody() = %q, want numeric query payload", string(bodyBytes))
	}
}
