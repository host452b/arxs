package model

import (
	"encoding/xml"
	"strings"
)

type AtomFeed struct {
	XMLName      xml.Name    `xml:"feed"`
	TotalResults int         `xml:"totalResults"`
	StartIndex   int         `xml:"startIndex"`
	ItemsPerPage int         `xml:"itemsPerPage"`
	Entries      []AtomEntry `xml:"entry"`
}

type AtomEntry struct {
	ID         string         `xml:"id"`
	Title      string         `xml:"title"`
	Summary    string         `xml:"summary"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Authors    []AtomAuthor   `xml:"author"`
	Links      []AtomLink     `xml:"link"`
	Categories []AtomCategory `xml:"category"`
}

type AtomAuthor struct {
	Name string `xml:"name"`
}

type AtomLink struct {
	Href  string `xml:"href,attr"`
	Rel   string `xml:"rel,attr"`
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr"`
}

type AtomCategory struct {
	Term string `xml:"term,attr"`
}

func ParseAtomFeed(data []byte) (*AtomFeed, error) {
	var feed AtomFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

// PDFLink returns the PDF URL from entry links.
func (e *AtomEntry) PDFLink() string {
	for _, l := range e.Links {
		if l.Title == "pdf" {
			return l.Href
		}
	}
	return ""
}

// extractArxivID extracts the arXiv ID from a full URL like
// "http://arxiv.org/abs/2401.12345v1" → "2401.12345"
func extractArxivID(rawID string) string {
	// Remove URL prefix
	id := rawID
	if idx := strings.LastIndex(id, "/abs/"); idx >= 0 {
		id = id[idx+5:]
	}
	// Remove version suffix (v1, v2, etc.)
	if idx := strings.LastIndex(id, "v"); idx > 0 {
		// Check that everything after 'v' is digits
		allDigits := true
		for _, c := range id[idx+1:] {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			id = id[:idx]
		}
	}
	return id
}

// ToPaper converts an AtomEntry to a Paper.
func (e *AtomEntry) ToPaper() Paper {
	arxivID := extractArxivID(e.ID)

	authors := make([]string, len(e.Authors))
	for i, a := range e.Authors {
		authors[i] = a.Name
	}

	cats := make([]string, len(e.Categories))
	for i, c := range e.Categories {
		cats[i] = c.Term
	}

	return Paper{
		ID:         arxivID,
		Title:      strings.TrimSpace(e.Title),
		Authors:    authors,
		Abstract:   strings.TrimSpace(e.Summary),
		Categories: cats,
		Published:  e.Published,
		Updated:    e.Updated,
		PDFUrl:     "https://arxiv.org/pdf/" + arxivID,
		HTMLUrl:    "https://arxiv.org/html/" + arxivID,
		AbsUrl:     "https://arxiv.org/abs/" + arxivID,
	}
}

// APIError checks if the feed contains an arXiv API error.
// Returns the error message or empty string if no error.
func (f *AtomFeed) APIError() string {
	if len(f.Entries) == 1 && strings.Contains(f.Entries[0].ID, "api/errors") {
		return strings.TrimSpace(f.Entries[0].Summary)
	}
	return ""
}
