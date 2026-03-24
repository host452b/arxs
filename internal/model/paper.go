package model

// Paper represents an arXiv paper with its metadata and links.
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
}

// SearchResult is the top-level JSON output structure.
type SearchResult struct {
	Query        QueryMeta `json:"query"`
	TotalResults int       `json:"total_results"`
	ReturnCount  int       `json:"return_count"`
	Papers       []Paper   `json:"papers"`
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
