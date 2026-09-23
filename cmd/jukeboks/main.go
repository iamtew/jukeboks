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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"jukeboks/internal/config"
	"jukeboks/internal/httplog"
	"jukeboks/internal/server"
	"jukeboks/internal/ytmd"
)

var (
	httpMu        sync.Mutex
	httpServer    *http.Server
	httpDone      chan error
	httpHandler   http.Handler
	preferredPort string
	actualPort    string
	webrootPath   string
	configPathStr string
)

func main() {
	port := flag.String("port", "42420", "HTTP server port")
	ytmdHost := flag.String("ytmd_host", "localhost", "YTMD host")
	ytmdPort := flag.String("ytmd_port", "26538", "YTMD port")
	webrootFlag := flag.String("webroot", "", "path to webroot directory")
	configFlag := flag.String("config", "", "path to jukeboks.json")
	noColor := flag.Bool("no-color", false, "disable colored HTTP log output")
	flag.Parse()
	attachParentConsole()
	httplog.Out = os.Stderr
	log.SetOutput(os.Stderr)

	if *noColor {
		httplog.UseColor = false
	}
	httplog.InitColor()

	webrootPath = resolvePath(*webrootFlag, "webroot")
	configPathStr = resolvePath(*configFlag, "jukeboks.json")

	if info, err := os.Stat(webrootPath); err != nil || !info.IsDir() {
		fatalf("webroot not found at %s", webrootPath)
	}

	store, err := config.NewStore(configPathStr)
	if err != nil {
		fatalf("failed to initialize config: %v", err)
	}
	configPathStr = store.Path()

	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(*ytmdHost, *ytmdPort)}
	client := ytmd.NewClient(target)
	httpHandler = server.New(store, client, webrootPath).Handler()
	preferredPort = *port

	if err := startHTTP(); err != nil {
		fatalf("failed to start server: %v", err)
	}
	waitForQuit()
}

func startHTTP() error {
	httpMu.Lock()
	defer httpMu.Unlock()
	if httpServer != nil {
		return nil
	}

	listener, port, err := listenWithFallback(preferredPort)
	if err != nil {
		return err
	}

	srv := &http.Server{Handler: httpHandler}
	done := make(chan error, 1)
	httpServer = srv
	httpDone = done
	actualPort = port

	go func() {
		err := srv.Serve(listener)
		if err != nil && !isNormalShutdownError(err) {
			log.Printf("server stopped unexpectedly: %v", err)
		}
		done <- err
	}()

	fmt.Fprintf(httplog.Out, "listening on http://localhost:%s\n", port)
	fmt.Fprintf(httplog.Out, "webroot: %s\n", webrootPath)
	fmt.Fprintf(httplog.Out, "config:  %s\n", configPathStr)
	return nil
}

func stopHTTP() {
	httpMu.Lock()
	srv := httpServer
	done := httpDone
	httpServer = nil
	httpDone = nil
	httpMu.Unlock()
	if srv == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	if done != nil {
		<-done
	}
}

func restartHTTP() {
	stopHTTP()
	if err := startHTTP(); err != nil {
		alert("Jukeboks", fmt.Sprintf("failed to restart server: %v", err))
	}
}

func servingURL(path string) string {
	httpMu.Lock()
	port := actualPort
	httpMu.Unlock()
	if port == "" {
		port = preferredPort
	}
	return "http://localhost:" + port + path
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
				fmt.Fprintf(httplog.Out, "port %d was busy, using %d instead\n", basePort, candidatePort)
			}
			return listener, strconv.Itoa(candidatePort), nil
		}
	}

	return nil, "", fmt.Errorf("unable to bind to port %s or any fallback ports", port)
}
