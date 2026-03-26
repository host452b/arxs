package zenodo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/host452b/arxs/v2/internal/provider"
	zenodoprovider "github.com/host452b/arxs/v2/internal/provider/zenodo"
)

func sampleZenodoResponse() []byte {
	resp := map[string]any{
		"hits": map[string]any{
			"total": 1,
			"hits": []map[string]any{
				{
					"id":  12345,
					"doi": "10.5281/zenodo.12345",
					"metadata": map[string]any{
						"title":            "Machine Learning Dataset",
						"creators":         []map[string]any{{"name": "Smith, John"}},
						"description":      "A dataset for ML research.",
						"publication_date": "2025-01-15",
						"resource_type":    map[string]any{"type": "dataset"},
					},
					"links": map[string]any{
						"html": "https://zenodo.org/record/12345",
					},
					"files": []map[string]any{
						{"key": "paper.pdf", "links": map[string]any{"self": "https://zenodo.org/record/12345/files/paper.pdf"}},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestZenodoProvider_Search_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(sampleZenodoResponse())
	}))
	defer srv.Close()

	p := zenodoprovider.New(zenodoprovider.WithBaseURL(srv.URL), zenodoprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "machine learning", Max: 5}, provider.SubjectFilter{ZenodoKeywords: []string{"machine learning"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
	if papers[0].Source != "zenodo" {
		t.Errorf("expected source=zenodo, got %s", papers[0].Source)
	}
	if papers[0].Title != "Machine Learning Dataset" {
		t.Errorf("unexpected title: %s", papers[0].Title)
	}
	if papers[0].DOI != "10.5281/zenodo.12345" {
		t.Errorf("unexpected DOI: %s", papers[0].DOI)
	}
}

// TestZenodoProvider_HTMLAbstract verifies that HTML tags are stripped from
// Zenodo's description field before being stored as Abstract.
func TestZenodoProvider_HTMLAbstract(t *testing.T) {
	resp := map[string]any{
		"hits": map[string]any{
			"total": 1,
			"hits": []map[string]any{
				{
					"id":  99999,
					"doi": "10.5281/zenodo.99999",
					"metadata": map[string]any{
						"title":            "Test Paper",
						"creators":         []map[string]any{{"name": "Doe, Jane"}},
						"description":      "<p>First paragraph.</p><p>Second paragraph with <em>emphasis</em>.</p>",
						"publication_date": "2024-06-01",
					},
					"links": map[string]any{"html": "https://zenodo.org/record/99999"},
					"files": []any{},
				},
			},
		},
	}
	data, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer srv.Close()

	p := zenodoprovider.New(zenodoprovider.WithBaseURL(srv.URL), zenodoprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "test", Max: 1}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
	abstract := papers[0].Abstract
	if containsHTML(abstract) {
		t.Errorf("Abstract still contains HTML tags: %q", abstract)
	}
	if abstract == "" {
		t.Error("Abstract is empty after stripping — should have text content")
	}
}

// TestZenodoProvider_URLFallback verifies that when links.html is empty,
// the provider constructs the URL from the record ID.
func TestZenodoProvider_URLFallback(t *testing.T) {
	resp := map[string]any{
		"hits": map[string]any{
			"total": 1,
			"hits": []map[string]any{
				{
					"id":  55555,
					"doi": "10.5281/zenodo.55555",
					"metadata": map[string]any{
						"title":            "No Link Record",
						"creators":         []map[string]any{{"name": "Author, A"}},
						"description":      "Abstract text.",
						"publication_date": "2024-01-01",
					},
					"links": map[string]any{"html": ""}, // empty — should fall back to ID-based URL
					"files": []any{},
				},
			},
		},
	}
	data, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer srv.Close()

	p := zenodoprovider.New(zenodoprovider.WithBaseURL(srv.URL), zenodoprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "test", Max: 1}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
	if papers[0].SourceURL == "" {
		t.Error("SourceURL is empty — should fall back to https://zenodo.org/records/{id}")
	}
	if papers[0].AbsUrl == "" {
		t.Error("AbsUrl is empty — should fall back to https://zenodo.org/records/{id}")
	}
}

func containsHTML(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '<' {
			for j := i + 1; j < len(s); j++ {
				if s[j] == '>' {
					return true
				}
			}
		}
	}
	return false
}

func TestZenodoProvider_Search_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", 400)
	}))
	defer srv.Close()

	p := zenodoprovider.New(zenodoprovider.WithBaseURL(srv.URL), zenodoprovider.WithRateInterval(0))
	_, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestZenodoProvider_Search_Empty(t *testing.T) {
	resp := map[string]any{"hits": map[string]any{"total": 0, "hits": []any{}}}
	data, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer srv.Close()

	p := zenodoprovider.New(zenodoprovider.WithBaseURL(srv.URL), zenodoprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 0 {
		t.Errorf("expected 0 papers, got %d", len(papers))
	}
}

func TestZenodoProvider_Search_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	p := zenodoprovider.New(zenodoprovider.WithBaseURL(srv.URL), zenodoprovider.WithRateInterval(0))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := p.Search(ctx, provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
