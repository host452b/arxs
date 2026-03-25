// internal/model/paper.go
package model

// Paper represents a research paper from any supported source.
type Paper struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Authors    []string `json:"authors"`
	Abstract   string   `json:"abstract"`
	Categories []string `json:"categories"`
	Published  string   `json:"published"`
	Updated    string   `json:"updated"`
	PDFUrl     string   `json:"pdf_url"`
	HTMLUrl    string   `json:"html_url"`
	AbsUrl     string   `json:"abs_url"`
	Citations  int      `json:"citations"`
	DOI        string   `json:"doi,omitempty"`
	Source     string   `json:"source"`      // "arxiv"|"zenodo"|"socarxiv"|"edarxiv"|"openalex"
	SourceURL  string   `json:"source_url"`  // canonical page URL on the source platform
}

// SearchResult is kept for backward compatibility (arXiv-only single-source).
type SearchResult struct {
	Query        QueryMeta `json:"query"`
	TotalResults int       `json:"total_results"`
	ReturnCount  int       `json:"return_count"`
	Papers       []Paper   `json:"papers"`
}

// MultiSourceResult is the top-level output for multi-source searches.
type MultiSourceResult struct {
	Query  QueryMeta     `json:"query"`
	Groups []SourceGroup `json:"groups"`
	Total  int           `json:"total"`
}

// SourceGroup holds results from one provider.
type SourceGroup struct {
	Source string  `json:"source"`
	Count  int     `json:"count"`
	Papers []Paper `json:"papers"`
}

// QueryMeta records the search parameters used.
type QueryMeta struct {
	Terms      map[string]string `json:"terms"`
	Subjects   []string          `json:"subjects"`
	Op         string            `json:"op"`
	From       string            `json:"from"`
	To         string            `json:"to"`
	Max        int               `json:"max"`
	SearchedAt string            `json:"searched_at"`
}

// AllPapers returns a flat slice of all papers across all groups, in display order.
func (r *MultiSourceResult) AllPapers() []Paper {
	var out []Paper
	for _, g := range r.Groups {
		out = append(out, g.Papers...)
	}
	return out
}
