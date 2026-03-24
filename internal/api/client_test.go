package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestClientSearch(t *testing.T) {
	// Serve fixture XML
	xmlData, err := os.ReadFile("../../testdata/sample_response.xml")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify User-Agent
		ua := r.Header.Get("User-Agent")
		if ua == "" || ua == "Go-http-client/1.1" {
			t.Errorf("expected custom User-Agent, got %q", ua)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write(xmlData)
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithRateInterval(1*time.Millisecond), // Fast for testing
	)

	params := QueryParams{
		Terms: map[string]string{"title": "transformer"},
		Max:   50,
	}

	result, err := client.Search(params)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if result.TotalResults != 5320 {
		t.Errorf("TotalResults = %d, want 5320", result.TotalResults)
	}
	if len(result.Papers) != 2 {
		t.Fatalf("len(Papers) = %d, want 2", len(result.Papers))
	}
	if result.Papers[0].ID != "2401.12345" {
		t.Errorf("Papers[0].ID = %q", result.Papers[0].ID)
	}
	if result.Papers[0].Title != "Attention Is All You Need Revisited" {
		t.Errorf("Papers[0].Title = %q", result.Papers[0].Title)
	}
}

func TestClientAPIError(t *testing.T) {
	errorXML := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/api/errors#bad_query</id>
    <title>Error</title>
    <summary>bad query syntax</summary>
  </entry>
</feed>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(errorXML))
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithRateInterval(1*time.Millisecond),
	)

	_, err := client.Search(QueryParams{Terms: map[string]string{"title": "test"}, Max: 10})
	if err == nil {
		t.Fatal("expected error from API error response")
	}
	if err.Error() != "arXiv API error: bad query syntax" {
		t.Errorf("error = %q", err.Error())
	}
}
