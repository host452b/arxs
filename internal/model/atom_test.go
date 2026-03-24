package model

import (
	"os"
	"testing"
)

func TestParseAtomFeed(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sample_response.xml")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	feed, err := ParseAtomFeed(data)
	if err != nil {
		t.Fatalf("ParseAtomFeed error: %v", err)
	}

	if feed.TotalResults != 5320 {
		t.Errorf("TotalResults = %d, want 5320", feed.TotalResults)
	}

	if len(feed.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(feed.Entries))
	}

	e := feed.Entries[0]
	if e.ID != "http://arxiv.org/abs/2401.12345v1" {
		t.Errorf("Entry[0].ID = %q", e.ID)
	}
	if e.Title != "Attention Is All You Need Revisited" {
		t.Errorf("Entry[0].Title = %q", e.Title)
	}
	if e.Summary != "We revisit the transformer architecture and propose improvements." {
		t.Errorf("Entry[0].Summary = %q", e.Summary)
	}
	if len(e.Authors) != 2 {
		t.Fatalf("Entry[0] authors = %d, want 2", len(e.Authors))
	}
	if e.Authors[0].Name != "A. Vaswani" {
		t.Errorf("Entry[0].Authors[0] = %q", e.Authors[0].Name)
	}
	if e.Published != "2024-01-15T00:00:00Z" {
		t.Errorf("Entry[0].Published = %q", e.Published)
	}

	// Check categories
	if len(e.Categories) != 2 {
		t.Fatalf("Entry[0] categories = %d, want 2", len(e.Categories))
	}
	if e.Categories[0].Term != "cs.AI" {
		t.Errorf("Entry[0].Categories[0].Term = %q", e.Categories[0].Term)
	}

	// Check links
	if e.PDFLink() != "http://arxiv.org/pdf/2401.12345v1" {
		t.Errorf("Entry[0].PDFLink() = %q", e.PDFLink())
	}
}

func TestAtomEntryToPaper(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sample_response.xml")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	feed, err := ParseAtomFeed(data)
	if err != nil {
		t.Fatalf("ParseAtomFeed error: %v", err)
	}

	paper := feed.Entries[0].ToPaper()

	if paper.ID != "2401.12345" {
		t.Errorf("ID = %q, want %q", paper.ID, "2401.12345")
	}
	if paper.Title != "Attention Is All You Need Revisited" {
		t.Errorf("Title = %q", paper.Title)
	}
	if len(paper.Authors) != 2 || paper.Authors[0] != "A. Vaswani" {
		t.Errorf("Authors = %v", paper.Authors)
	}
	if paper.PDFUrl != "https://arxiv.org/pdf/2401.12345" {
		t.Errorf("PDFUrl = %q", paper.PDFUrl)
	}
	if paper.HTMLUrl != "https://arxiv.org/html/2401.12345" {
		t.Errorf("HTMLUrl = %q", paper.HTMLUrl)
	}
	if paper.AbsUrl != "https://arxiv.org/abs/2401.12345" {
		t.Errorf("AbsUrl = %q", paper.AbsUrl)
	}
	if len(paper.Categories) != 2 || paper.Categories[0] != "cs.AI" {
		t.Errorf("Categories = %v", paper.Categories)
	}
}

func TestDetectAPIError(t *testing.T) {
	errorXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/api/errors#incorrect_id_format_for_1234</id>
    <title>Error</title>
    <summary>incorrect id format for '1234'</summary>
  </entry>
</feed>`)

	feed, err := ParseAtomFeed(errorXML)
	if err != nil {
		t.Fatalf("ParseAtomFeed error: %v", err)
	}

	apiErr := feed.APIError()
	if apiErr == "" {
		t.Fatal("expected API error, got empty string")
	}
	if apiErr != "incorrect id format for '1234'" {
		t.Errorf("APIError() = %q", apiErr)
	}
}
