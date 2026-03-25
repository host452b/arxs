package zenodo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/host452b/arxs/internal/provider"
	zenodoprovider "github.com/host452b/arxs/internal/provider/zenodo"
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
