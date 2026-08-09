package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
)

type responseEnvelope struct {
	ExitCode int    `json:"exitCode"`
	Message  string `json:"message,omitempty"`
	Data     any    `json:"data,omitempty"`
}

func main() {
	port := flag.String("port", "42420", "HTTP server port")
	ytmdHost := flag.String("ytmd_host", "localhost", "YTMD host")
	ytmdPort := flag.String("ytmd_port", "26538", "YTMD port")
	flag.Parse()

	target := &url.URL{Scheme: "http", Host: netJoin(*ytmdHost, *ytmdPort)}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, responseEnvelope{ExitCode: 0, Message: "ok"})
	})

	mux.HandleFunc("/cmd/ytmd/", func(w http.ResponseWriter, r *http.Request) {
		proxyToYTMD(target, w, r)
	})

	mux.HandleFunc("/cmd/ytmd", func(w http.ResponseWriter, r *http.Request) {
		proxyToYTMD(target, w, r)
	})

	mux.HandleFunc("/cmd/jb/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, responseEnvelope{ExitCode: 0, Message: "custom jukeboks command scaffold"})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join("webroot", "index.html"))
			return
		}

		if strings.HasPrefix(r.URL.Path, "/admin") || strings.HasPrefix(r.URL.Path, "/overlay") {
			http.FileServer(http.Dir("webroot")).ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join("webroot", strings.TrimPrefix(r.URL.Path, "/")))
	})

	listener, actualPort, err := listenWithFallback(*port)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	server := &http.Server{Handler: mux}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	fmt.Printf("jukeboks listening on http://localhost:%s\n", actualPort)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped unexpectedly: %v", err)
	}
}

func listenWithFallback(port string) (net.Listener, string, error) {
	basePort, err := strconv.Atoi(port)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port %q: %w", port, err)
	}

	for attempt := 0; attempt < 10; attempt++ {
		candidatePort := basePort + attempt
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", candidatePort))
		if err == nil {
			if candidatePort != basePort {
				fmt.Printf("port %d was busy, using %d instead\n", basePort, candidatePort)
			}
			return listener, strconv.Itoa(candidatePort), nil
		}
	}

	return nil, "", fmt.Errorf("unable to bind to port %s or any fallback ports", port)
}

func proxyToYTMD(target *url.URL, w http.ResponseWriter, r *http.Request) {
	upstreamURL := *target
	upstreamURL.Path = ytmdRouteForPath(r.URL.Path)
	upstreamURL.RawQuery = r.URL.RawQuery

	method := resolveUpstreamMethod(r.Method, r.URL.Path)
	if method == http.MethodGet && r.URL.Query().Get("_method") != "" {
		method = strings.ToUpper(r.URL.Query().Get("_method"))
	}
	bodyReader, err := buildUpstreamBody(method, r)
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to build upstream request body: %v", err)})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), method, upstreamURL.String(), bodyReader)
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to build upstream request: %v", err)})
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if bodyReader != nil && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(responseEnvelope{
			ExitCode: 1,
			Message:  fmt.Sprintf("failed to reach YTMD at %s: %v", target.Host, err),
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, responseEnvelope{ExitCode: 1, Message: fmt.Sprintf("failed to read YTMD response: %v", err)})
		return
	}

	if resp.StatusCode >= 400 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(responseEnvelope{
			ExitCode: 1,
			Message:  fmt.Sprintf("YTMD returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))),
		})
		return
	}

	var decoded any
	if len(bytes.TrimSpace(bodyBytes)) > 0 {
		if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
			decoded = string(bodyBytes)
		}
	}

	payload := responseEnvelope{ExitCode: 0, Data: decoded}
	if decoded == nil {
		payload.Message = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}

func ytmdRouteForPath(path string) string {
	path = strings.TrimPrefix(path, "/cmd/ytmd")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return "/api/v1/"
	}

	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) > 1 {
		last := strings.ToLower(segments[len(segments)-1])
		if isSupportedMethod(last) {
			segments = segments[:len(segments)-1]
		}
	}

	trimmedPath := "/" + strings.Join(segments, "/")
	if trimmedPath == "/" {
		return "/api/v1/"
	}

	if strings.HasPrefix(trimmedPath, "/queue/") {
		return "/api/v1/queue/" + strings.TrimPrefix(trimmedPath, "/queue/")
	}

	return "/api/v1" + trimmedPath
}

func resolveUpstreamMethod(requestMethod, path string) string {
	commandPath := strings.TrimPrefix(path, "/cmd/ytmd")
	commandPath = strings.TrimSuffix(commandPath, "/")
	if commandPath == "" {
		return requestMethod
	}

	segments := strings.Split(strings.TrimPrefix(commandPath, "/"), "/")
	if len(segments) > 0 {
		last := strings.ToLower(segments[len(segments)-1])
		if isSupportedMethod(last) {
			method := strings.ToUpper(last)
			if method == http.MethodGet {
				return requestMethod
			}
			if method == http.MethodPost {
				return method
			}
			return method
		}
	}

	if commandPath == "/queue/{index}" || commandPath == "/queue/index" {
		if requestMethod == http.MethodGet {
			return http.MethodPatch
		}
		return requestMethod
	}

	switch commandPath {
	case "/play", "/pause", "/toggle-play", "/previous", "/next", "/seek-to", "/go-back", "/go-forward", "/toggle-mute", "/switch-repeat", "/like", "/dislike", "/volume", "/fullscreen":
		return http.MethodPost
	case "/shuffle":
		if requestMethod == http.MethodPost {
			return http.MethodPost
		}
		return http.MethodGet
	case "/queue":
		if requestMethod == http.MethodPost || requestMethod == http.MethodPatch || requestMethod == http.MethodDelete {
			return requestMethod
		}
		return http.MethodGet
	default:
		return requestMethod
	}
}

func isSupportedMethod(segment string) bool {
	switch strings.ToLower(segment) {
	case "get", "post", "put", "patch", "delete":
		return true
	default:
		return false
	}
}

func buildUpstreamBody(method string, r *http.Request) (io.Reader, error) {
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
		return r.Body, nil
	}

	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(bodyBytes)) > 0 {
			return bytes.NewReader(bodyBytes), nil
		}
	}

	queryValues := r.URL.Query()
	if len(queryValues) == 0 {
		return nil, nil
	}

	payload := make(map[string]any, len(queryValues))
	for key, values := range queryValues {
		if len(values) == 1 {
			payload[key] = coerceQueryValue(values[0])
			continue
		}
		payload[key] = values
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(bodyBytes), nil
}

func coerceQueryValue(value string) any {
	if value == "" {
		return value
	}

	if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
		return strings.EqualFold(value, "true")
	}

	if parsedInt, err := strconv.ParseInt(value, 10, 64); err == nil {
		return parsedInt
	}

	if parsedFloat, err := strconv.ParseFloat(value, 64); err == nil {
		return parsedFloat
	}

	return value
}

func netJoin(host, port string) string {
	return host + ":" + port
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
