package openalex_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/host452b/arxs/v2/internal/provider"
	oap "github.com/host452b/arxs/v2/internal/provider/openalex"
)

func sampleOpenAlexResponse() []byte {
	resp := map[string]any{
		"results": []map[string]any{
			{
				"id":    "https://openalex.org/W2741809807",
				"doi":   "https://doi.org/10.1016/j.econ.2025.01.003",
				"title": "Economic Impacts of AI",
				"authorships": []map[string]any{
					{"author": map[string]any{"display_name": "Smith, Jane"}},
				},
				"abstract_inverted_index": map[string]any{
					"We":      []int{0},
					"study":   []int{1},
					"AI":      []int{2},
					"impacts": []int{3},
				},
				"publication_date": "2025-01-20",
				"primary_location": map[string]any{
					"landing_page_url": "https://doi.org/10.1016/j.econ.2025.01.003",
					"pdf_url":          nil,
				},
				"best_oa_location": map[string]any{
					"pdf_url": "https://repo.edu/paper.pdf",
				},
				"cited_by_count": 42,
			},
		},
		"meta": map[string]any{"count": 1},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestOpenAlexProvider_Search_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(sampleOpenAlexResponse())
	}))
	defer srv.Close()

	p := oap.New(oap.WithBaseURL(srv.URL), oap.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "AI", Max: 5}, provider.SubjectFilter{OpenAlexConcepts: []string{"C162324750"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
	if papers[0].Source != "openalex" {
		t.Errorf("expected source=openalex, got %s", papers[0].Source)
	}
	if papers[0].Citations != 42 {
		t.Errorf("expected citations=42, got %d", papers[0].Citations)
	}
	if papers[0].Abstract == "" {
		t.Error("expected reconstructed abstract, got empty")
	}
	if papers[0].PDFUrl != "https://repo.edu/paper.pdf" {
		t.Errorf("unexpected pdf_url: %s", papers[0].PDFUrl)
	}
}

func TestOpenAlexProvider_Search_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", 429)
	}))
	defer srv.Close()

	p := oap.New(oap.WithBaseURL(srv.URL), oap.WithRateInterval(0))
	_, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAlexProvider_Search_Empty(t *testing.T) {
	resp := map[string]any{"results": []any{}, "meta": map[string]any{"count": 0}}
	data, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()

	p := oap.New(oap.WithBaseURL(srv.URL), oap.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 0 {
		t.Errorf("expected 0, got %d", len(papers))
	}
}

func TestOpenAlexProvider_Search_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	p := oap.New(oap.WithBaseURL(srv.URL), oap.WithRateInterval(0))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := p.Search(ctx, provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
