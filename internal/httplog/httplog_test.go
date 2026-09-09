package httplog

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestShouldLogPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/cmd/jb/songinfo", true},
		{"/cmd/ytmd/play", true},
		{"/api/config", true},
		{"/health", true},
		{"/", false},
		{"/admin/index.html", false},
		{"/overlay/app.js", false},
	}

	for _, tc := range tests {
		if got := ShouldLogPath(tc.path); got != tc.want {
			t.Fatalf("ShouldLogPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestJBCommandName(t *testing.T) {
	if got := JBCommandName("/cmd/jb/songrequest/"); got != "songrequest" {
		t.Fatalf("JBCommandName() = %q, want songrequest", got)
	}
	if got := JBCommandName("/cmd/ytmd/play"); got != "" {
		t.Fatalf("JBCommandName() = %q, want empty", got)
	}
}

func TestLogRequestWithoutColor(t *testing.T) {
	var buf bytes.Buffer
	oldOut := Out
	oldUseColor := UseColor
	oldEnabled := Enabled
	Out = &buf
	UseColor = false
	Enabled = true
	t.Cleanup(func() {
		Out = oldOut
		UseColor = oldUseColor
		Enabled = oldEnabled
	})

	LogRequest(http.MethodGet, "/cmd/jb/songinfo", "", http.StatusOK, 12*time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "GET") {
		t.Fatalf("output missing method: %q", output)
	}
	if !strings.Contains(output, "/cmd/jb/songinfo") {
		t.Fatalf("output missing path: %q", output)
	}
	if !strings.Contains(output, "200") {
		t.Fatalf("output missing status: %q", output)
	}
	if !strings.Contains(output, "12ms") {
		t.Fatalf("output missing duration: %q", output)
	}
}

func TestLogUpstream(t *testing.T) {
	var buf bytes.Buffer
	oldOut := Out
	oldUseColor := UseColor
	oldEnabled := Enabled
	Out = &buf
	UseColor = false
	Enabled = true
	t.Cleanup(func() {
		Out = oldOut
		UseColor = oldUseColor
		Enabled = oldEnabled
	})

	LogUpstream(http.MethodPost, "http://localhost:26538/api/v1/search", 200, 5*time.Millisecond)
	if !strings.Contains(buf.String(), "POST") || !strings.Contains(buf.String(), "/api/v1/search") {
		t.Fatalf("unexpected upstream output: %q", buf.String())
	}
}

func TestLogJBResult(t *testing.T) {
	var buf bytes.Buffer
	oldOut := Out
	oldUseColor := UseColor
	oldEnabled := Enabled
	Out = &buf
	UseColor = false
	Enabled = true
	t.Cleanup(func() {
		Out = oldOut
		UseColor = oldUseColor
		Enabled = oldEnabled
	})

	LogJBResult(1, "Song already in queue.", map[string]any{"reason": "duplicate"})
	output := buf.String()
	if !strings.Contains(output, "exitCode=1") {
		t.Fatalf("output missing exit code: %q", output)
	}
	if !strings.Contains(output, "Song already in queue.") {
		t.Fatalf("output missing message: %q", output)
	}
	if !strings.Contains(output, "reason=duplicate") {
		t.Fatalf("output missing data summary: %q", output)
	}
}

func TestSummarizeDataEmpty(t *testing.T) {
	if got := summarizeData(nil); got != "" {
		t.Fatalf("summarizeData(nil) = %q, want empty", got)
	}
}
