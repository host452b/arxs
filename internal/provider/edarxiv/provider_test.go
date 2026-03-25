package edarxiv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/host452b/arxs/internal/provider"
	edarxivprovider "github.com/host452b/arxs/internal/provider/edarxiv"
)

func sampleOSFResponse() []byte {
	resp := map[string]any{
		"data": []map[string]any{
			{
				"id":   "xyz99",
				"type": "preprints",
				"attributes": map[string]any{
					"title":          "Educational Outcomes in Digital Learning",
					"description":    "We study digital learning outcomes.",
					"date_published": "2025-03-05T00:00:00Z",
					"doi":            "10.35542/osf.io/xyz99",
				},
				"links": map[string]any{
					"html": "https://osf.io/preprints/edarxiv/xyz99",
				},
			},
		},
		"links": map[string]any{"next": nil},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestEdArxivProvider_Search_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.Write(sampleOSFResponse())
	}))
	defer srv.Close()

	p := edarxivprovider.New(edarxivprovider.WithBaseURL(srv.URL), edarxivprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "digital learning", Max: 5}, provider.SubjectFilter{OSFProviders: []string{"edarxiv"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
	if papers[0].Source != "edarxiv" {
		t.Errorf("expected source=edarxiv, got %s", papers[0].Source)
	}
}

func TestEdArxivProvider_Search_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", 403)
	}))
	defer srv.Close()

	p := edarxivprovider.New(edarxivprovider.WithBaseURL(srv.URL), edarxivprovider.WithRateInterval(0))
	_, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEdArxivProvider_Search_Empty(t *testing.T) {
	resp := map[string]any{"data": []any{}, "links": map[string]any{"next": nil}}
	data, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()

	p := edarxivprovider.New(edarxivprovider.WithBaseURL(srv.URL), edarxivprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 0 {
		t.Errorf("expected 0, got %d", len(papers))
	}
}

func TestEdArxivProvider_Search_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	p := edarxivprovider.New(edarxivprovider.WithBaseURL(srv.URL), edarxivprovider.WithRateInterval(0))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := p.Search(ctx, provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
