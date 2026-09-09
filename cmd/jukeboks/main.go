package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"jukeboks/internal/config"
	"jukeboks/internal/httplog"
	"jukeboks/internal/server"
	"jukeboks/internal/ytmd"
)

func main() {
	port := flag.String("port", "42420", "HTTP server port")
	ytmdHost := flag.String("ytmd_host", "localhost", "YTMD host")
	ytmdPort := flag.String("ytmd_port", "26538", "YTMD port")
	webrootFlag := flag.String("webroot", "", "path to webroot directory")
	configFlag := flag.String("config", "", "path to jukeboks.json")
	noColor := flag.Bool("no-color", false, "disable colored HTTP log output")
	flag.Parse()

	if *noColor {
		httplog.UseColor = false
	}
	httplog.InitColor()

	webroot := resolvePath(*webrootFlag, "webroot")
	configPath := resolvePath(*configFlag, "jukeboks.json")

	if info, err := os.Stat(webroot); err != nil || !info.IsDir() {
		log.Fatalf("webroot not found at %s", webroot)
	}

	store, err := config.NewStore(configPath)
	if err != nil {
		log.Fatalf("failed to initialize config: %v", err)
	}

	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(*ytmdHost, *ytmdPort)}
	client := ytmd.NewClient(target)
	srv := server.New(store, client, webroot)

	listener, actualPort, err := listenWithFallback(*port)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	httpServer := &http.Server{Handler: srv.Handler()}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		httplog.ShutdownTerminal()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	listenURL := fmt.Sprintf("http://localhost:%s", actualPort)
	httplog.SetupTerminal(listenURL, func(pollCtx context.Context) (httplog.SongStatus, error) {
		payload, err := client.FetchJSONWithRetry(pollCtx, "/api/v1/song", 1, 0)
		if err != nil {
			return httplog.SongStatus{}, err
		}
		song := ytmd.ParseCurrentSong(payload)
		return httplog.SongStatus{
			Title:  song.Title,
			Artist: song.Artist,
			State:  song.State,
		}, nil
	})

	fmt.Fprintf(httplog.Out, "webroot: %s\n", webroot)
	fmt.Fprintf(httplog.Out, "config:  %s\n", store.Path())
	if err := httpServer.Serve(listener); err != nil && !isNormalShutdownError(err) {
		log.Fatalf("server stopped unexpectedly: %v", err)
	}
}

func resolvePath(explicit, name string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	if dir := deployedDir(); dir != "" {
		return filepath.Join(dir, name)
	}
	return filepath.Join(".", name)
}

func deployedDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	if info, err := os.Stat(filepath.Join(dir, "webroot")); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

func isNormalShutdownError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "use of closed network connection") || strings.Contains(message, "closed")
}

func listenWithFallback(port string) (net.Listener, string, error) {
	basePort, err := strconv.Atoi(port)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port %q: %w", port, err)
	}

	for attempt := 0; attempt < 10; attempt++ {
		candidatePort := basePort + attempt
		listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", candidatePort))
		if err == nil {
			if candidatePort != basePort {
				fmt.Printf("port %d was busy, using %d instead\n", basePort, candidatePort)
			}
			return listener, strconv.Itoa(candidatePort), nil
		}
	}

	return nil, "", fmt.Errorf("unable to bind to port %s or any fallback ports", port)
}
