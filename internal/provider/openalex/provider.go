package openalex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/host452b/arxs/internal/api"
	"github.com/host452b/arxs/internal/model"
	"github.com/host452b/arxs/internal/provider"
)

const defaultBaseURL = "https://api.openalex.org"

type Option func(*Provider)

func WithBaseURL(u string) Option { return func(p *Provider) { p.baseURL = u } }
func WithRateInterval(d time.Duration) Option {
	return func(p *Provider) { p.rateLimiter = api.NewRateLimiter(d) }
}

type Provider struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *api.RateLimiter
}

func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: api.NewRateLimiter(100 * time.Millisecond),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) ID() provider.ProviderID { return provider.ProviderOpenAlex }

type openAlexResponse struct {
	Results []struct {
		ID          string `json:"id"`
		DOI         string `json:"doi"`
		Title       string `json:"title"`
		Authorships []struct {
			Author struct {
				DisplayName string `json:"display_name"`
			} `json:"author"`
		} `json:"authorships"`
		AbstractInvertedIndex map[string][]int `json:"abstract_inverted_index"`
		PublicationDate       string           `json:"publication_date"`
		PrimaryLocation       *struct {
			LandingPageURL string  `json:"landing_page_url"`
			PDFURL         *string `json:"pdf_url"`
		} `json:"primary_location"`
		BestOALocation *struct {
			PDFURL *string `json:"pdf_url"`
		} `json:"best_oa_location"`
		CitedByCount int `json:"cited_by_count"`
	} `json:"results"`
}

func (p *Provider) Search(ctx context.Context, q provider.Query, f provider.SubjectFilter) ([]model.Paper, error) {
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	params := url.Values{}
	var filters []string
	for _, c := range f.OpenAlexConcepts {
		filters = append(filters, "concepts.id:"+c)
	}
	filters = append(filters, "is_oa:true")
	params.Set("filter", strings.Join(filters, ","))
	if q.Keywords != "" {
		params.Set("search", q.Keywords)
	}
	params.Set("per_page", fmt.Sprintf("%d", q.Max))
	params.Set("mailto", "arxs")

	reqURL := p.baseURL + "/works?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("openalex: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openalex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openalex: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openalex: reading response: %w", err)
	}

	var oar openAlexResponse
	if err := json.Unmarshal(body, &oar); err != nil {
		return nil, fmt.Errorf("openalex: parsing JSON: %w", err)
	}

	papers := make([]model.Paper, 0, len(oar.Results))
	for _, r := range oar.Results {
		authors := make([]string, len(r.Authorships))
		for i, a := range r.Authorships {
			authors[i] = a.Author.DisplayName
		}

		abstract := reconstructAbstract(r.AbstractInvertedIndex)

		pdfURL := ""
		if r.BestOALocation != nil && r.BestOALocation.PDFURL != nil {
			pdfURL = *r.BestOALocation.PDFURL
		} else if r.PrimaryLocation != nil && r.PrimaryLocation.PDFURL != nil {
			pdfURL = *r.PrimaryLocation.PDFURL
		}

		landingURL := ""
		if r.PrimaryLocation != nil {
			landingURL = r.PrimaryLocation.LandingPageURL
		}

		doi := strings.TrimPrefix(r.DOI, "https://doi.org/")
		workID := strings.TrimPrefix(r.ID, "https://openalex.org/")

		papers = append(papers, model.Paper{
			ID:        "openalex." + workID,
			Title:     r.Title,
			Authors:   authors,
			Abstract:  abstract,
			Published: r.PublicationDate,
			DOI:       doi,
			PDFUrl:    pdfURL,
			AbsUrl:    landingURL,
			Citations: r.CitedByCount,
			Source:    "openalex",
			SourceURL: landingURL,
		})
	}
	return papers, nil
}

// reconstructAbstract converts OpenAlex inverted index back to a string.
func reconstructAbstract(index map[string][]int) string {
	if len(index) == 0 {
		return ""
	}
	type wordPos struct {
		word string
		pos  int
	}
	var pairs []wordPos
	for word, positions := range index {
		for _, pos := range positions {
			pairs = append(pairs, wordPos{word, pos})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].pos < pairs[j].pos })
	words := make([]string, len(pairs))
	for i, wp := range pairs {
		words[i] = wp.word
	}
	return strings.Join(words, " ")
}

func (p *Provider) DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error) {
	if paper.PDFUrl == "" {
		return nil, fmt.Errorf("openalex: no direct PDF for %s — visit %s", paper.ID, paper.SourceURL)
	}
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", paper.PDFUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("openalex: creating download request: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openalex: download request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openalex: download HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openalex: reading download body: %w", err)
	}
	return data, nil
}
