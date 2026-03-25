// internal/provider/provider.go
package provider

import (
	"context"

	"github.com/joejiang/arxs/internal/model"
)

// ProviderID identifies a paper source.
type ProviderID string

const (
	ProviderArxiv    ProviderID = "arxiv"
	ProviderZenodo   ProviderID = "zenodo"
	ProviderSocArxiv ProviderID = "socarxiv"
	ProviderEdArxiv  ProviderID = "edarxiv"
	ProviderOpenAlex ProviderID = "openalex"
)

// Query carries search parameters for all providers.
// Terms/Op are arXiv-specific; Keywords is a pre-built string for other providers.
type Query struct {
	Terms     map[string]string // arXiv field-specific: "title"→expr, "abs"→expr, etc.
	Op        string            // "and"|"or" between Terms (arXiv only)
	Keywords  string            // flattened keyword string for non-arXiv sources
	From      string            // YYYY-MM-DD
	To        string            // YYYY-MM-DD
	Max       int
	SortBy    string
	SortOrder string
}

// SubjectFilter carries per-source subject filter parameters.
type SubjectFilter struct {
	ArxivCats        []string // arXiv: ["cs.AI","cs.LG"]
	OpenAlexConcepts []string // OpenAlex concept IDs: ["C41008148"]
	ZenodoKeywords   []string // Zenodo subject keywords: ["machine learning"]
	OSFProviders     []string // "socarxiv", "edarxiv", or both
	OSFSubjects      []string // OSF subject display strings
}

// Provider is the interface all paper sources must implement.
type Provider interface {
	ID() ProviderID
	Search(ctx context.Context, q Query, f SubjectFilter) ([]model.Paper, error)
	DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error)
}
