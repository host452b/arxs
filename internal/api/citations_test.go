package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joejiang/arxs/internal/model"
)

func TestFetchCitations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect POST to /paper/batch
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		resp := []struct {
			PaperID       string `json:"paperId"`
			CitationCount int    `json:"citationCount"`
		}{
			{"abc123", 500},
			{"def456", 42},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	papers := []model.Paper{
		{ID: "2401.12345", Title: "Paper A"},
		{ID: "2402.67890", Title: "Paper B"},
	}

	cf := NewCitationFetcher(
		WithCitationBaseURL(server.URL),
		WithCitationRateInterval(1*time.Millisecond),
	)

	err := cf.FetchCitations(papers)
	if err != nil {
		t.Fatalf("FetchCitations error: %v", err)
	}

	if papers[0].Citations != 500 {
		t.Errorf("Paper A citations = %d, want 500", papers[0].Citations)
	}
	if papers[1].Citations != 42 {
		t.Errorf("Paper B citations = %d, want 42", papers[1].Citations)
	}
}

func TestFetchCitationsEmpty(t *testing.T) {
	cf := NewCitationFetcher(WithCitationRateInterval(1 * time.Millisecond))
	err := cf.FetchCitations(nil)
	if err != nil {
		t.Fatalf("expected no error for empty papers, got: %v", err)
	}
}
