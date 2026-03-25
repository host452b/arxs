package socarxiv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/host452b/arxs/internal/provider"
	socarxivprovider "github.com/host452b/arxs/internal/provider/socarxiv"
)

func sampleOSFResponse() []byte {
	resp := map[string]any{
		"data": []map[string]any{
			{
				"id":   "abc12",
				"type": "preprints",
				"attributes": map[string]any{
					"title":          "Social Inequality in Networks",
					"description":    "We study social network inequality.",
					"date_published": "2025-02-10T00:00:00Z",
					"doi":            "10.31235/osf.io/abc12",
				},
				"links": map[string]any{
					"html": "https://osf.io/preprints/socarxiv/abc12",
				},
			},
		},
		"links": map[string]any{"next": nil},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestSocArxivProvider_Search_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.Write(sampleOSFResponse())
	}))
	defer srv.Close()

	p := socarxivprovider.New(socarxivprovider.WithBaseURL(srv.URL), socarxivprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "inequality", Max: 5}, provider.SubjectFilter{OSFProviders: []string{"socarxiv"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
	if papers[0].Source != "socarxiv" {
		t.Errorf("expected source=socarxiv, got %s", papers[0].Source)
	}
}

// TestSocArxivProvider_AuthorsParsed verifies that contributor names are populated
// when the API returns embedded contributors (embed=contributors).
func TestSocArxivProvider_AuthorsParsed(t *testing.T) {
	resp := map[string]any{
		"data": []map[string]any{
			{
				"id":   "xyz99",
				"type": "preprints",
				"attributes": map[string]any{
					"title":          "Test Paper With Authors",
					"description":    "Abstract text.",
					"date_published": "2025-06-01T00:00:00Z",
					"doi":            "10.31235/osf.io/xyz99",
				},
				"links": map[string]any{
					"html": "https://osf.io/preprints/socarxiv/xyz99",
				},
				"embeds": map[string]any{
					"contributors": map[string]any{
						"data": []map[string]any{
							{
								"id": "xyz99-user1",
								"attributes": map[string]any{
									"bibliographic":           true,
									"unregistered_contributor": nil,
								},
								"embeds": map[string]any{
									"users": map[string]any{
										"data": map[string]any{
											"id":         "user1",
											"attributes": map[string]any{"full_name": "Alice Smith"},
										},
									},
								},
							},
							{
								"id": "xyz99-user2",
								"attributes": map[string]any{
									"bibliographic":           true,
									"unregistered_contributor": "Bob Jones (Unregistered)",
								},
								"embeds": map[string]any{
									"users": map[string]any{
										"data": map[string]any{
											"id":         "user2",
											"attributes": map[string]any{"full_name": "Bob Jones"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"links": map[string]any{"next": nil},
	}
	data, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request includes embed=contributors
		if r.URL.Query().Get("embed") != "contributors" {
			t.Errorf("request missing embed=contributors, got: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.Write(data)
	}))
	defer srv.Close()

	p := socarxivprovider.New(socarxivprovider.WithBaseURL(srv.URL), socarxivprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "test", Max: 5}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
	authors := papers[0].Authors
	if len(authors) != 2 {
		t.Fatalf("expected 2 authors, got %d: %v", len(authors), authors)
	}
	if authors[0] != "Alice Smith" {
		t.Errorf("author[0] = %q, want %q", authors[0], "Alice Smith")
	}
	if authors[1] != "Bob Jones (Unregistered)" {
		t.Errorf("author[1] = %q, want %q", authors[1], "Bob Jones (Unregistered)")
	}
}

func TestSocArxivProvider_Search_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", 403)
	}))
	defer srv.Close()

	p := socarxivprovider.New(socarxivprovider.WithBaseURL(srv.URL), socarxivprovider.WithRateInterval(0))
	_, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSocArxivProvider_Search_Empty(t *testing.T) {
	resp := map[string]any{"data": []any{}, "links": map[string]any{"next": nil}}
	data, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()

	p := socarxivprovider.New(socarxivprovider.WithBaseURL(srv.URL), socarxivprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 0 {
		t.Errorf("expected 0, got %d", len(papers))
	}
}

func TestSocArxivProvider_Search_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	p := socarxivprovider.New(socarxivprovider.WithBaseURL(srv.URL), socarxivprovider.WithRateInterval(0))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := p.Search(ctx, provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
