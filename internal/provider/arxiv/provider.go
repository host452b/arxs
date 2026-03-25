// internal/provider/arxiv/provider.go
package arxiv

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/joejiang/arxs/internal/api"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/provider"
)

const defaultBaseURL = "https://export.arxiv.org/api/query"

// Option configures the arXiv provider.
type Option func(*Provider)

// WithBaseURL overrides the API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(p *Provider) { p.baseURL = url }
}

// WithRateInterval overrides the rate limiter interval (for testing).
func WithRateInterval(d time.Duration) Option {
	return func(p *Provider) { p.rateLimiter = api.NewRateLimiter(d) }
}

// Provider implements provider.Provider for arXiv.
type Provider struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *api.RateLimiter
}

// New creates an arXiv Provider.
func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: api.NewRateLimiter(3 * time.Second),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) ID() provider.ProviderID { return provider.ProviderArxiv }

func (p *Provider) Search(ctx context.Context, q provider.Query, f provider.SubjectFilter) ([]model.Paper, error) {
	p.rateLimiter.Wait()

	// Build arXiv QueryParams from provider.Query + SubjectFilter
	params := api.QueryParams{
		Terms:     q.Terms,
		Subjects:  f.ArxivCats,
		Op:        q.Op,
		From:      q.From,
		To:        q.To,
		Max:       q.Max,
		SortBy:    q.SortBy,
		SortOrder: q.SortOrder,
	}
	// Fallback: if no Terms, use Keywords as all-field search
	if len(params.Terms) == 0 && q.Keywords != "" {
		params.Terms = map[string]string{"all": q.Keywords}
	}

	queryURL := api.BuildQueryURL(params)
	// Replace host if baseURL is overridden for testing
	if p.baseURL != defaultBaseURL {
		rest := strings.SplitN(queryURL, "?", 2)
		if len(rest) == 2 {
			queryURL = p.baseURL + "?" + rest[1]
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("arxiv: creating request: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("arxiv: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("arxiv: reading response: %w", err)
	}

	feed, err := model.ParseAtomFeed(body)
	if err != nil {
		return nil, fmt.Errorf("arxiv: parsing XML: %w", err)
	}
	if apiErr := feed.APIError(); apiErr != "" {
		return nil, fmt.Errorf("arxiv: API error: %s", apiErr)
	}

	papers := make([]model.Paper, len(feed.Entries))
	for i, e := range feed.Entries {
		papers[i] = e.ToPaper()
		papers[i].Source = "arxiv"
		papers[i].SourceURL = papers[i].AbsUrl
	}
	return papers, nil
}

func (p *Provider) DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error) {
	if paper.PDFUrl == "" {
		return nil, fmt.Errorf("arxiv: no PDF URL for %s", paper.ID)
	}
	p.rateLimiter.Wait()
	req, err := http.NewRequestWithContext(ctx, "GET", paper.PDFUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", api.UserAgent)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv: download HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
