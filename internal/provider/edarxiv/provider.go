package edarxiv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/host452b/arxs/v2/internal/api"
	"github.com/host452b/arxs/v2/internal/model"
	"github.com/host452b/arxs/v2/internal/provider"
)

const defaultBaseURL = "https://api.osf.io/v2"
const providerID = "edarxiv"

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

func (p *Provider) ID() provider.ProviderID { return provider.ProviderEdArxiv }

type osfResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Title         string `json:"title"`
			Description   string `json:"description"`
			DatePublished string `json:"date_published"`
			DOI           string `json:"doi"`
		} `json:"attributes"`
		Links struct {
			HTML string `json:"html"`
		} `json:"links"`
		Embeds struct {
			Contributors struct {
				Data []struct {
					Attributes struct {
						Bibliographic           bool    `json:"bibliographic"`
						UnregisteredContributor *string `json:"unregistered_contributor"`
					} `json:"attributes"`
					Embeds struct {
						Users struct {
							Data struct {
								Attributes struct {
									FullName string `json:"full_name"`
								} `json:"attributes"`
							} `json:"data"`
						} `json:"users"`
					} `json:"embeds"`
				} `json:"data"`
			} `json:"contributors"`
		} `json:"embeds"`
	} `json:"data"`
}

func (p *Provider) Search(ctx context.Context, q provider.Query, f provider.SubjectFilter) ([]model.Paper, error) {
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("filter[provider]", providerID)
	if q.Keywords != "" {
		params.Set("filter[title]", q.Keywords)
	}
	params.Set("page[size]", fmt.Sprintf("%d", q.Max))
	params.Set("embed", "contributors")

	reqURL := p.baseURL + "/preprints/?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("edarxiv: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("edarxiv: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("edarxiv: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("edarxiv: reading response: %w", err)
	}

	var osfResp osfResponse
	if err := json.Unmarshal(body, &osfResp); err != nil {
		return nil, fmt.Errorf("edarxiv: parsing JSON: %w", err)
	}

	papers := make([]model.Paper, 0, len(osfResp.Data))
	for _, d := range osfResp.Data {
		published := d.Attributes.DatePublished
		if len(published) > 10 {
			published = published[:10]
		}
		pageURL := d.Links.HTML
		if pageURL == "" {
			pageURL = "https://osf.io/preprints/" + providerID + "/" + d.ID
		}
		var authors []string
		for _, c := range d.Embeds.Contributors.Data {
			if !c.Attributes.Bibliographic {
				continue
			}
			name := ""
			if c.Attributes.UnregisteredContributor != nil && *c.Attributes.UnregisteredContributor != "" {
				name = *c.Attributes.UnregisteredContributor
			} else {
				name = c.Embeds.Users.Data.Attributes.FullName
			}
			if name != "" {
				authors = append(authors, name)
			}
		}
		papers = append(papers, model.Paper{
			ID:        providerID + "." + d.ID,
			Title:     d.Attributes.Title,
			Authors:   authors,
			Abstract:  d.Attributes.Description,
			Published: published,
			DOI:       d.Attributes.DOI,
			AbsUrl:    pageURL,
			Source:    "edarxiv",
			SourceURL: pageURL,
		})
	}
	return papers, nil
}

func (p *Provider) DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error) {
	if paper.PDFUrl == "" {
		return nil, fmt.Errorf("edarxiv: no direct PDF URL for %s — visit %s", paper.ID, paper.SourceURL)
	}
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", paper.PDFUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("edarxiv: creating download request: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("edarxiv: download request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("edarxiv: download HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("edarxiv: reading download body: %w", err)
	}
	return data, nil
}
