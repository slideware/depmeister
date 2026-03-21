package depsdev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL = "https://api.deps.dev"
	defaultTimeout = 10 * time.Second
	maxRetries     = 3
	initialBackoff = 1 * time.Second
)

// Client is the deps.dev API client with concurrency limiting and retry.
type Client struct {
	baseURL    string
	httpClient *http.Client
	sem        chan struct{}
}

// NewClient creates a new deps.dev API client.
func NewClient(concurrency int, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if concurrency < 1 {
		concurrency = 1
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		sem: make(chan struct{}, concurrency),
	}
}

// GetVersion fetches version info for a package.
func (c *Client) GetVersion(ctx context.Context, system, name, version string) (*VersionResponse, error) {
	path := fmt.Sprintf("/v3/systems/%s/packages/%s/versions/%s",
		url.PathEscape(system),
		url.PathEscape(name),
		url.PathEscape(version),
	)

	var resp VersionResponse
	if err := c.doJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetProject fetches project info (including scorecard).
func (c *Client) GetProject(ctx context.Context, projectID string) (*ProjectResponse, error) {
	path := fmt.Sprintf("/v3/projects/%s", url.PathEscape(projectID))

	var resp ProjectResponse
	if err := c.doJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doJSON(ctx context.Context, path string, dst any) error {
	// Acquire semaphore.
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	reqURL := c.baseURL + path

	var lastErr error
	backoff := initialBackoff

	for attempt := range maxRetries {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing request: %w", err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("reading response body: %w", readErr)
		}

		if resp.StatusCode == http.StatusOK {
			if err := json.Unmarshal(body, dst); err != nil {
				return fmt.Errorf("decoding JSON from %s: %w", path, err)
			}
			return nil
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			slog.Debug("rate limited, backing off",
				"url", reqURL,
				"attempt", attempt+1,
				"backoff", backoff,
			)
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, path)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff *= 2
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("not found: %s", path)
		}

		return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, path, string(body))
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
