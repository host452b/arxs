package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joejiang/arxs/internal/model"
)

func TestWriteAndReadResults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")

	result := &model.SearchResult{
		Query: model.QueryMeta{
			Terms:      map[string]string{"title": "transformer"},
			Subjects:   []string{"cs"},
			Op:         "and",
			Max:        50,
			SearchedAt: "2025-03-24T10:00:00Z",
		},
		TotalResults: 100,
		ReturnCount:  2,
		Papers: []model.Paper{
			{
				ID:         "2401.12345",
				Title:      "Test Paper 1",
				Authors:    []string{"Author A"},
				Abstract:   "Abstract 1",
				Categories: []string{"cs.AI"},
				Published:  "2024-01-15T00:00:00Z",
				PDFUrl:     "https://arxiv.org/pdf/2401.12345",
				HTMLUrl:    "https://arxiv.org/html/2401.12345",
				AbsUrl:     "https://arxiv.org/abs/2401.12345",
			},
			{
				ID:    "2402.67890",
				Title: "Test Paper 2",
			},
		},
	}

	// Write
	err := WriteResults(path, result)
	if err != nil {
		t.Fatalf("WriteResults error: %v", err)
	}

	// File should exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file not created")
	}

	// Read back
	got, err := ReadResults(path)
	if err != nil {
		t.Fatalf("ReadResults error: %v", err)
	}

	if got.TotalResults != 100 {
		t.Errorf("TotalResults = %d, want 100", got.TotalResults)
	}
	if got.ReturnCount != 2 {
		t.Errorf("ReturnCount = %d, want 2", got.ReturnCount)
	}
	if len(got.Papers) != 2 {
		t.Fatalf("len(Papers) = %d, want 2", len(got.Papers))
	}
	if got.Papers[0].ID != "2401.12345" {
		t.Errorf("Papers[0].ID = %q", got.Papers[0].ID)
	}
	if got.Papers[0].Title != "Test Paper 1" {
		t.Errorf("Papers[0].Title = %q", got.Papers[0].Title)
	}
	if got.Query.Terms["title"] != "transformer" {
		t.Errorf("Query.Terms = %v", got.Query.Terms)
	}
}

func TestReadResultsNotFound(t *testing.T) {
	_, err := ReadResults("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
