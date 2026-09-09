package ytmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func RouteForPath(path string) string {
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

func ResolveUpstreamMethod(requestMethod, path string) string {
	return resolveUpstreamMethod(requestMethod, path)
}

func isSupportedMethod(segment string) bool {
	switch strings.ToLower(segment) {
	case "get", "post", "put", "patch", "delete":
		return true
	default:
		return false
	}
}

func BuildUpstreamBody(method string, r *http.Request) (io.Reader, error) {
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
