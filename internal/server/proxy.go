package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"jukeboks/internal/policy"
	"jukeboks/internal/ytmd"
)

var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailers":            {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
	"Host":                {},
}

func (s *Server) proxyToYTMD(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Config.Get()
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to load config: %v", err)})
		return
	}
	if err := policy.Enforce(cfg, r); err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: err.Error()})
		return
	}

	upstreamURL := *s.YTMD.Base
	upstreamURL.Path = ytmd.RouteForPath(r.URL.Path)
	upstreamURL.RawQuery = r.URL.RawQuery

	method := ytmd.ResolveUpstreamMethod(r.Method, r.URL.Path)
	if method == http.MethodGet && r.URL.Query().Get("_method") != "" {
		method = strings.ToUpper(r.URL.Query().Get("_method"))
	}
	bodyReader, err := ytmd.BuildUpstreamBody(method, r)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to build upstream request body: %v", err)})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), method, upstreamURL.String(), bodyReader)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to build upstream request: %v", err)})
		return
	}

	copyHeaders(r.Header, req.Header)
	if bodyReader != nil && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.YTMD.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(Envelope{
			ExitCode: 1,
			Message:  fmt.Sprintf("failed to reach YTMD at %s: %v", s.YTMD.Base.Host, err),
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, Envelope{ExitCode: 1, Message: fmt.Sprintf("failed to read YTMD response: %v", err)})
		return
	}

	if resp.StatusCode >= 400 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(Envelope{
			ExitCode: 1,
			Message:  fmt.Sprintf("YTMD returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))),
		})
		return
	}

	var decoded any
	if len(strings.TrimSpace(string(bodyBytes))) > 0 {
		if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
			decoded = string(bodyBytes)
		}
	}

	payload := Envelope{ExitCode: 0, Data: decoded}
	if decoded == nil {
		payload.Message = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}

func copyHeaders(src, dst http.Header) {
	extraHopByHop := map[string]struct{}{}
	for _, value := range src.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			name := http.CanonicalHeaderKey(strings.TrimSpace(token))
			if name != "" {
				extraHopByHop[name] = struct{}{}
			}
		}
	}

	for key, values := range src {
		canonical := http.CanonicalHeaderKey(key)
		if _, drop := hopByHopHeaders[canonical]; drop {
			continue
		}
		if _, drop := extraHopByHop[canonical]; drop {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
