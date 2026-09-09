package httplog

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	Out      io.Writer = os.Stderr
	Enabled  bool      = true
	UseColor bool      = true

	colorInit sync.Once
)

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	cyan    = "\033[36m"
	magenta = "\033[35m"
	white   = "\033[37m"
)

func SetEnabled(enabled bool) {
	Enabled = enabled
}

func InitColor() {
	colorInit.Do(func() {
		enableNativeConsole()
		if !isTerminal(Out) {
			UseColor = false
		}
	})
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func ShouldLogPath(path string) bool {
	return strings.HasPrefix(path, "/cmd/") ||
		strings.HasPrefix(path, "/api/") ||
		path == "/health" ||
		path == "/health/"
}

func LogRequestStart(method, path, query string) {
	if !Enabled {
		return
	}
	InitColor()

	displayPath := path
	if query != "" {
		displayPath = path + "?" + query
	}

	fmt.Fprintf(Out, "%s  %s  %s\n",
		colorize(dim, timestamp()),
		colorize(methodColor(method), method),
		colorize(bold+white, displayPath),
	)
}

func LogRequest(method, path, query string, status int, duration time.Duration) {
	if !Enabled {
		return
	}
	InitColor()

	displayPath := path

	fmt.Fprintf(Out, "%s  %s  %s  %s  %s\n",
		colorize(dim, timestamp()),
		colorize(methodColor(method), method),
		colorize(bold+white, displayPath),
		colorize(statusColor(status), fmt.Sprintf("%d", status)),
		colorize(dim, formatDuration(duration)),
	)
}

func LogUpstream(method, url string, status int, duration time.Duration) {
	if !Enabled {
		return
	}
	writeUpstream(method, url, status, duration)
}

func writeUpstream(method, url string, status int, duration time.Duration) {
	InitColor()

	statusText := "-"
	statusStyle := dim
	if status > 0 {
		statusText = fmt.Sprintf("%d", status)
		statusStyle = statusColor(status)
	}

	fmt.Fprintf(Out, "    -> %s %s  %s  %s\n",
		colorize(methodColor(method), method),
		url,
		colorize(statusStyle, statusText),
		colorize(dim, formatDuration(duration)),
	)
}

func LogJBStart(command string, fields map[string]string) {
	if !Enabled {
		return
	}
	InitColor()

	fmt.Fprintf(Out, "  %s: %s\n",
		colorize(magenta, "command"),
		command,
	)
	for key, value := range fields {
		if strings.TrimSpace(value) == "" {
			continue
		}
		fmt.Fprintf(Out, "  %s: %s\n",
			colorize(magenta, key),
			value,
		)
	}
}

func LogJBResult(exitCode int, message string, data any) {
	if !Enabled {
		return
	}
	InitColor()

	exitStyle := green
	if exitCode != 0 {
		exitStyle = yellow
	}

	line := fmt.Sprintf("exitCode=%d", exitCode)
	if strings.TrimSpace(message) != "" {
		line += "  " + message
	}
	if summary := summarizeData(data); summary != "" {
		line += "  " + summary
	}

	fmt.Fprintf(Out, "  %s: %s\n",
		colorize(magenta, "result"),
		colorize(exitStyle, line),
	)
}

func summarizeData(data any) string {
	m, ok := data.(map[string]any)
	if !ok || len(m) == 0 {
		return ""
	}

	parts := make([]string, 0, len(m))
	for key, value := range m {
		if value == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}
	return strings.Join(parts, " ")
}

func JBCommandName(path string) string {
	path = strings.TrimSuffix(strings.TrimSpace(path), "/")
	if !strings.HasPrefix(path, "/cmd/jb/") {
		return ""
	}
	name := strings.TrimPrefix(path, "/cmd/jb/")
	if idx := strings.Index(name, "/"); idx >= 0 {
		name = name[:idx]
	}
	return name
}

func JBFields(r *http.Request) map[string]string {
	fields := map[string]string{}
	if input := strings.TrimSpace(r.URL.Query().Get("input")); input != "" {
		fields["input"] = input
	}
	return fields
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Round(time.Millisecond).String()
}

func methodColor(method string) string {
	switch method {
	case http.MethodGet:
		return cyan
	case http.MethodPost:
		return green
	case http.MethodPut:
		return yellow
	case http.MethodDelete:
		return red
	default:
		return white
	}
}

func statusColor(status int) string {
	switch {
	case status >= 500:
		return red
	case status >= 400:
		return yellow
	case status >= 300:
		return cyan
	case status >= 200:
		return green
	default:
		return white
	}
}

func colorize(style, text string) string {
	if !UseColor || style == "" {
		return text
	}
	return style + text + reset
}
