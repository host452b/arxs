package zenodo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/joejiang/arxs/internal/api"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/provider"
)

const defaultBaseURL = "https://zenodo.org/api"

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
		rateLimiter: api.NewRateLimiter(1 * time.Second),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) ID() provider.ProviderID { return provider.ProviderZenodo }

type zenodoResponse struct {
	Hits struct {
		Total int `json:"total"`
		Hits  []struct {
			ID       int    `json:"id"`
			DOI      string `json:"doi"`
			Metadata struct {
				Title           string `json:"title"`
				Description     string `json:"description"`
				PublicationDate string `json:"publication_date"`
				Creators        []struct {
					Name string `json:"name"`
				} `json:"creators"`
			} `json:"metadata"`
			Links struct {
				HTML string `json:"html"`
			} `json:"links"`
			Files []struct {
				Key   string `json:"key"`
				Links struct {
					Self string `json:"self"`
				} `json:"links"`
			} `json:"files"`
		} `json:"hits"`
	} `json:"hits"`
}

func (p *Provider) Search(ctx context.Context, q provider.Query, f provider.SubjectFilter) ([]model.Paper, error) {
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	kw := q.Keywords
	if len(f.ZenodoKeywords) > 0 && kw != "" {
		kw = strings.Join(f.ZenodoKeywords, " OR ") + " AND " + kw
	} else if len(f.ZenodoKeywords) > 0 {
		kw = strings.Join(f.ZenodoKeywords, " OR ")
	}

	params := url.Values{}
	params.Set("q", kw)
	params.Set("type", "publication")
	params.Set("size", fmt.Sprintf("%d", q.Max))
	if q.From != "" && q.To != "" {
		params.Set("publication_date", q.From+"/"+q.To)
	}

	reqURL := p.baseURL + "/records?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("zenodo: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zenodo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zenodo: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("zenodo: reading response: %w", err)
	}

	var zr zenodoResponse
	if err := json.Unmarshal(body, &zr); err != nil {
		return nil, fmt.Errorf("zenodo: parsing JSON: %w", err)
	}

	papers := make([]model.Paper, 0, len(zr.Hits.Hits))
	for _, h := range zr.Hits.Hits {
		authors := make([]string, len(h.Metadata.Creators))
		for i, c := range h.Metadata.Creators {
			authors[i] = c.Name
		}
		pdfURL := ""
		for _, f := range h.Files {
			if strings.HasSuffix(strings.ToLower(f.Key), ".pdf") {
				pdfURL = f.Links.Self
				break
			}
		}
		papers = append(papers, model.Paper{
			ID:        fmt.Sprintf("zenodo.%d", h.ID),
			Title:     h.Metadata.Title,
			Authors:   authors,
			Abstract:  h.Metadata.Description,
			Published: h.Metadata.PublicationDate,
			DOI:       h.DOI,
			PDFUrl:    pdfURL,
			AbsUrl:    h.Links.HTML,
			Source:    "zenodo",
			SourceURL: h.Links.HTML,
		})
	}
	return papers, nil
}

func (p *Provider) DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error) {
	if paper.PDFUrl == "" {
		return nil, fmt.Errorf("zenodo: no PDF URL for %s", paper.ID)
	}
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", paper.PDFUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("zenodo: creating download request: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zenodo: download request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zenodo: download HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("zenodo: reading download body: %w", err)
	}
	return data, nil
}
