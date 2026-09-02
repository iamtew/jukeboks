package httplog

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	saveCursor    = "\033[s"
	restoreCursor = "\033[u"
)

type SongStatus struct {
	Title  string
	Artist string
	State  string
}

type terminalWriter struct {
	mu  sync.Mutex
	out io.Writer
}

func (w *terminalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.out.Write(p)
}

var (
	terminalMu       sync.Mutex
	terminalActive   bool
	terminalOut      *terminalWriter
	terminalHeight   int
	terminalWidth    int
	headerListenURL  string
	headerPoll       func(context.Context) (SongStatus, error)
	headerStop       context.CancelFunc
)

func SetupTerminal(listenURL string, poll func(context.Context) (SongStatus, error)) {
	terminalMu.Lock()
	defer terminalMu.Unlock()

	if terminalActive || !Enabled {
		return
	}

	enableNativeConsole()
	InitColor()

	file, ok := Out.(*os.File)
	if !ok {
		file = os.Stderr
	}
	if !isTerminal(file) {
		return
	}

	width, height, sized := terminalSize()
	if !sized {
		height = 25
		width = 120
	}

	terminalOut = &terminalWriter{out: file}
	Out = terminalOut
	terminalHeight = height
	terminalWidth = width
	headerListenURL = listenURL
	headerPoll = poll
	terminalActive = true

	drawHeader(SongStatus{State: "connecting"})
	fmt.Fprintf(Out, "\033[2;%d;r\033[2;1H", terminalHeight)

	ctx, cancel := context.WithCancel(context.Background())
	headerStop = cancel
	go runHeaderPoller(ctx)

	if headerPoll != nil {
		pollCtx, cancelPoll := context.WithTimeout(context.Background(), 3*time.Second)
		status, err := headerPoll(pollCtx)
		cancelPoll()
		if err != nil {
			status = SongStatus{State: "no player"}
		}
		drawHeader(status)
	}
}

func ShutdownTerminal() {
	terminalMu.Lock()
	defer terminalMu.Unlock()
	if headerStop != nil {
		headerStop()
		headerStop = nil
	}
	if terminalActive {
		fmt.Fprint(Out, "\033[r")
		terminalActive = false
	}
}

func runHeaderPoller(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if headerPoll == nil {
				continue
			}
			pollCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			status, err := headerPoll(pollCtx)
			cancel()
			if err != nil {
				status = SongStatus{State: "no player"}
			}
			drawHeader(status)
		}
	}
}

func drawHeader(status SongStatus) {
	if !terminalActive || terminalOut == nil {
		return
	}

	terminalOut.mu.Lock()
	defer terminalOut.mu.Unlock()

	line := formatHeader(headerListenURL, status)
	if terminalWidth > 0 && len([]rune(stripANSI(line))) > terminalWidth {
		line = truncateANSI(line, terminalWidth)
	}

	fmt.Fprintf(terminalOut.out, "%s\033[1;1H\033[2K%s%s", saveCursor, line, restoreCursor)
}

func formatHeader(listenURL string, status SongStatus) string {
	sep := colorize(dim, " · ")
	parts := []string{
		colorize(bold+magenta, "Jukeboks!"),
		colorize(cyan, listenURL),
	}

	switch strings.ToLower(strings.TrimSpace(status.State)) {
	case "playing":
		parts = append(parts, colorize(green, "playing"))
	case "paused":
		parts = append(parts, colorize(yellow, "paused"))
	case "connecting":
		parts = append(parts, colorize(dim, "connecting"))
	default:
		parts = append(parts, colorize(dim, "no player"))
	}

	track := formatTrack(status.Title, status.Artist)
	if track != "" {
		parts = append(parts, colorize(white, track))
	}

	return strings.Join(parts, sep)
}

func formatTrack(title, artist string) string {
	title = strings.TrimSpace(title)
	artist = strings.TrimSpace(artist)
	switch {
	case title != "" && artist != "":
		return title + " — " + artist
	case title != "":
		return title
	case artist != "":
		return artist
	default:
		return ""
	}
}

func stripANSI(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	escaping := false
	for i := 0; i < len(text); i++ {
		if text[i] == '\033' {
			escaping = true
			continue
		}
		if escaping {
			if text[i] == 'm' {
				escaping = false
			}
			continue
		}
		b.WriteByte(text[i])
	}
	return b.String()
}

func truncateANSI(text string, maxWidth int) string {
	plain := stripANSI(text)
	if len([]rune(plain)) <= maxWidth {
		return text
	}
	if maxWidth <= 1 {
		return "…"
	}
	runes := []rune(plain)
	return string(runes[:maxWidth-1]) + "…"
}
