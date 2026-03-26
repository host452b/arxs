package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/host452b/arxs/v2/internal/model"
)

const (
	defaultCitationBaseURL = "https://api.semanticscholar.org/graph/v1"
	citationRateInterval   = 1 * time.Second
)

// CitationFetcher fetches citation counts from Semantic Scholar.
type CitationFetcher struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *RateLimiter
}

// CitationOption configures the CitationFetcher.
type CitationOption func(*CitationFetcher)

func WithCitationBaseURL(url string) CitationOption {
	return func(cf *CitationFetcher) { cf.baseURL = url }
}

func WithCitationRateInterval(d time.Duration) CitationOption {
	return func(cf *CitationFetcher) { cf.rateLimiter = NewRateLimiter(d) }
}

func NewCitationFetcher(opts ...CitationOption) *CitationFetcher {
	cf := &CitationFetcher{
		baseURL:     defaultCitationBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: NewRateLimiter(citationRateInterval),
	}
	for _, opt := range opts {
		opt(cf)
	}
	return cf
}

// batchRequest is the request body for Semantic Scholar batch API.
type batchRequest struct {
	IDs []string `json:"ids"`
}

// batchResponseItem is one item from the batch response.
type batchResponseItem struct {
	PaperID       string `json:"paperId"`
	CitationCount int    `json:"citationCount"`
}

// FetchCitations fills in the Citations field for each paper using Semantic Scholar.
// Papers are queried in batches of up to 500.
func (cf *CitationFetcher) FetchCitations(papers []model.Paper) error {
	if len(papers) == 0 {
		return nil
	}

	const batchSize = 500

	// Build arXiv ID to paper index mapping
	idxMap := make(map[string]int)
	for i := range papers {
		idxMap[papers[i].ID] = i
	}

	for start := 0; start < len(papers); start += batchSize {
		end := start + batchSize
		if end > len(papers) {
			end = len(papers)
		}

		batch := papers[start:end]
		ids := make([]string, len(batch))
		for i, p := range batch {
			ids[i] = "ARXIV:" + p.ID
		}

		if err := cf.rateLimiter.Wait(context.Background()); err != nil {
			return err
		}

		results, err := cf.fetchBatch(ids)
		if err != nil {
			return err
		}

		// Match results back to papers by position (batch API preserves order)
		for i, item := range results {
			idx := start + i
			if idx < len(papers) {
				papers[idx].Citations = item.CitationCount
			}
		}
	}

	return nil
}

func (cf *CitationFetcher) fetchBatch(ids []string) ([]batchResponseItem, error) {
	body, err := json.Marshal(batchRequest{IDs: ids})
	if err != nil {
		return nil, err
	}

	url := cf.baseURL + "/paper/batch?fields=citationCount"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := cf.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Semantic Scholar API error: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var results []batchResponseItem
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}
