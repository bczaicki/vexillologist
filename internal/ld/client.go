package ld

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	// DefaultBaseURL is the public LaunchDarkly API host.
	DefaultBaseURL = "https://app.launchdarkly.com"

	// apiVersion pins the LD REST API version via the LD-API-Version header.
	apiVersion = "20240415"

	// contentTypeSemanticPatch marks a PATCH body as semantic-patch
	// instructions rather than JSON Patch.
	contentTypeSemanticPatch = "application/json; domain-model=launchdarkly.semanticpatch"

	// pageLimit is the page size used for paginated list endpoints.
	pageLimit = 100
)

// API is the surface both the CLI and TUI depend on; *Client implements it,
// and tests substitute an in-memory fake.
type API interface {
	ListProjects(ctx context.Context) ([]Project, error)
	ListEnvironments(ctx context.Context, projectKey string) ([]Environment, error)
	ListFlags(ctx context.Context, projectKey string, opts ListFlagsOptions) ([]FeatureFlag, error)
	GetFlag(ctx context.Context, projectKey, flagKey string, envKeys []string) (*FeatureFlag, error)
	CreateFlag(ctx context.Context, projectKey string, params CreateFlagParams) (*FeatureFlag, error)
	ToggleFlag(ctx context.Context, projectKey, flagKey, envKey string, on bool, comment string) (*FeatureFlag, error)
	ArchiveFlag(ctx context.Context, projectKey, flagKey, comment string) (*FeatureFlag, error)
	RestoreFlag(ctx context.Context, projectKey, flagKey, comment string) (*FeatureFlag, error)
	GetFlagStatuses(ctx context.Context, projectKey, envKey string) (map[string]FlagStatus, error)
}

// Client is a minimal LaunchDarkly REST API client covering the endpoints
// vexillologist needs.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	// sleep is swapped out in tests to avoid real rate-limit waits.
	sleep func(time.Duration)
}

var _ API = (*Client)(nil)

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API host (tests, federal instances).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient overrides the underlying *http.Client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// NewClient returns a Client authenticated with the given API access token.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		sleep:      time.Sleep,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// do issues one API request, retrying once on 429 per the X-Ratelimit-Reset
// header, and decodes a 2xx body into out (when out is non-nil).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, contentType string, body, out any) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	for attempt := 0; ; attempt++ {
		var reader io.Reader
		if bodyBytes != nil {
			reader = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, reader)
		if err != nil {
			return err
		}
		// LD expects the raw token, not "Bearer <token>".
		req.Header.Set("Authorization", c.token)
		req.Header.Set("LD-API-Version", apiVersion)
		if bodyBytes != nil {
			if contentType == "" {
				contentType = "application/json"
			}
			req.Header.Set("Content-Type", contentType)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			wait := rateLimitWait(resp.Header)
			io.Copy(io.Discard, resp.Body) //nolint:errcheck
			resp.Body.Close()              //nolint:errcheck
			c.sleep(wait)
			continue
		}

		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			apiErr := &APIError{StatusCode: resp.StatusCode}
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			_ = json.Unmarshal(data, apiErr)
			return apiErr
		}

		if out == nil {
			io.Copy(io.Discard, resp.Body) //nolint:errcheck
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
		return nil
	}
}

// rateLimitWait derives how long to wait from X-Ratelimit-Reset (epoch
// milliseconds), clamped to [1s, 10s].
func rateLimitWait(h http.Header) time.Duration {
	const (
		min = time.Second
		max = 10 * time.Second
	)
	resetMs, err := strconv.ParseInt(h.Get("X-Ratelimit-Reset"), 10, 64)
	if err != nil {
		return min
	}
	wait := time.Until(time.UnixMilli(resetMs))
	if wait < min {
		return min
	}
	if wait > max {
		return max
	}
	return wait
}
