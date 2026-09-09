package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"jukeboks/internal/httplog"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status      int
	body        bytes.Buffer
	captureBody bool
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	if w.captureBody {
		_, _ = w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !httplog.ShouldLogPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		jbCommand := httplog.JBCommandName(r.URL.Path)
		isJB := jbCommand != ""

		httplog.LogRequestStart(r.Method, r.URL.Path, r.URL.RawQuery)
		if isJB {
			httplog.LogJBStart(jbCommand, httplog.JBFields(r))
		}

		lw := &loggingResponseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
			captureBody:    isJB,
		}

		next.ServeHTTP(lw, r)

		if isJB && lw.body.Len() > 0 {
			var env Envelope
			if err := json.Unmarshal(lw.body.Bytes(), &env); err == nil {
				httplog.LogJBResult(env.ExitCode, env.Message, env.Data)
			}
		}

		status := lw.status
		if status == 0 {
			status = http.StatusOK
		}
		httplog.LogRequest(r.Method, normalizeLoggedPath(r.URL.Path), "", status, time.Since(start))
	})
}

func normalizeLoggedPath(path string) string {
	return strings.TrimSuffix(path, "/")
}
