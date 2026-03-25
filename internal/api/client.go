package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/host452b/arxs/internal/model"
)

const (
	defaultBaseURL      = "https://export.arxiv.org/api/query"
	defaultRateInterval = 3 * time.Second
	UserAgent           = "arxs/2.0.1 (https://github.com/host452b/arxs)"
)

// Client is an arXiv API client with rate limiting and custom User-Agent.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *RateLimiter
}

// Option configures the Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithRateInterval overrides the rate limit interval (for testing).
func WithRateInterval(d time.Duration) Option {
	return func(c *Client) { c.rateLimiter = NewRateLimiter(d) }
}

// NewClient creates an arXiv API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: NewRateLimiter(defaultRateInterval),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Search executes a search query and returns parsed results.
func (c *Client) Search(params QueryParams) (*model.SearchResult, error) {
	if err := c.rateLimiter.Wait(context.Background()); err != nil {
		return nil, err
	}

	queryURL := BuildQueryURL(params)

	// If baseURL is overridden (testing), replace the host
	if c.baseURL != defaultBaseURL {
		queryURL = c.baseURL + "?" + queryURL[len("https://export.arxiv.org/api/query?"):]
	}

	req, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	feed, err := model.ParseAtomFeed(body)
	if err != nil {
		return nil, fmt.Errorf("parsing XML: %w", err)
	}

	// Check for API error
	if apiErr := feed.APIError(); apiErr != "" {
		return nil, fmt.Errorf("arXiv API error: %s", apiErr)
	}

	// Convert to Papers
	papers := make([]model.Paper, len(feed.Entries))
	for i, entry := range feed.Entries {
		papers[i] = entry.ToPaper()
	}

	result := &model.SearchResult{
		Query: model.QueryMeta{
			Terms:      params.Terms,
			Subjects:   params.Subjects,
			Op:         params.Op,
			Max:        params.Max,
			SearchedAt: time.Now().UTC().Format(time.RFC3339),
		},
		TotalResults: feed.TotalResults,
		ReturnCount:  len(papers),
		Papers:       papers,
	}

	return result, nil
}

// DownloadFile downloads a file from the given URL and returns its content.
func (c *Client) DownloadFile(url string) ([]byte, error) {
	if err := c.rateLimiter.Wait(context.Background()); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
