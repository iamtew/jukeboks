package ytmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const defaultTimeout = 5 * time.Second

type Client struct {
	Base *url.URL
	HTTP *http.Client
}

func NewClient(base *url.URL) *Client {
	return &Client{
		Base: base,
		HTTP: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   2 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          10,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 5 * time.Second,
			},
		},
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.HTTP.Do(req)
}

func (c *Client) FetchJSONWithRetry(ctx context.Context, endpoint string, attempts int, delay time.Duration) (any, error) {
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base.String()+endpoint, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if attempt < attempts-1 {
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("endpoint returned %d", resp.StatusCode)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if attempt < attempts-1 {
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}

		var payload any
		decoder := json.NewDecoder(resp.Body)
		err = decoder.Decode(&payload)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to decode payload: %w", err)
			if attempt < attempts-1 {
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}

		return payload, nil
	}

	return nil, lastErr
}

func (c *Client) PostJSON(ctx context.Context, endpoint string, body any) (any, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base.String()+endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if len(bodyBytes) > 0 {
			return nil, fmt.Errorf("endpoint returned %d: %s", resp.StatusCode, string(bodyBytes))
		}
		return nil, fmt.Errorf("endpoint returned %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return map[string]any{}, nil
	}

	var decoded any
	if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	return decoded, nil
}

func fetchYTMDJSONWithRetry(target *url.URL, endpoint string, attempts int, delay time.Duration) (any, error) {
	return NewClient(target).FetchJSONWithRetry(context.Background(), endpoint, attempts, delay)
}
