package api

import (
	"net/url"
	"testing"
)

func TestBuildQuerySimple(t *testing.T) {
	params := QueryParams{
		Terms: map[string]string{"title": "transformer"},
		Max:   50,
	}
	u := BuildQueryURL(params)

	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("invalid URL: %v", err)
	}

	if parsed.Host != "export.arxiv.org" {
		t.Errorf("host = %q", parsed.Host)
	}

	q := parsed.Query()
	sq := q.Get("search_query")
	if sq != "ti:transformer" {
		t.Errorf("search_query = %q, want %q", sq, "ti:transformer")
	}
	if q.Get("max_results") != "50" {
		t.Errorf("max_results = %q", q.Get("max_results"))
	}
}

func TestBuildQueryMultipleFields(t *testing.T) {
	params := QueryParams{
		Terms: map[string]string{
			"title":  "transformer or attention",
			"author": "vaswani",
		},
		Op:  "and",
		Max: 50,
	}
	u := BuildQueryURL(params)
	parsed, _ := url.Parse(u)
	sq := parsed.Query().Get("search_query")

	// Should contain both title and author parts joined by AND
	if sq == "" {
		t.Fatal("search_query is empty")
	}
	// The exact format depends on the parser output, but it should contain AND
	if !containsAll(sq, "ti:transformer", "OR", "ti:attention", "AND", "au:vaswani") {
		t.Errorf("search_query = %q, missing expected parts", sq)
	}
}

func TestBuildQueryWithSubjects(t *testing.T) {
	params := QueryParams{
		Terms:    map[string]string{"title": "LLM"},
		Subjects: []string{"cs", "stat"},
		Max:      50,
	}
	u := BuildQueryURL(params)
	parsed, _ := url.Parse(u)
	sq := parsed.Query().Get("search_query")

	if !containsAll(sq, "ti:LLM", "cat:cs.*", "cat:stat.*") {
		t.Errorf("search_query = %q, missing expected parts", sq)
	}
}

func TestBuildQueryWithDates(t *testing.T) {
	params := QueryParams{
		Terms: map[string]string{"title": "LLM"},
		From:  "2024-01",
		To:    "2025-03",
		Max:   50,
	}
	u := BuildQueryURL(params)
	parsed, _ := url.Parse(u)
	sq := parsed.Query().Get("search_query")

	if !containsAll(sq, "submittedDate:[202401010000", "TO", "202503312359]") {
		t.Errorf("search_query = %q, missing date filter", sq)
	}
}

func TestBuildQueryAllFields(t *testing.T) {
	params := QueryParams{
		Terms: map[string]string{"all": "transformer"},
		Max:   50,
	}
	u := BuildQueryURL(params)
	parsed, _ := url.Parse(u)
	sq := parsed.Query().Get("search_query")

	if !containsAll(sq, "ti:transformer", "abs:transformer", "au:transformer") {
		t.Errorf("search_query = %q, missing all-fields expansion", sq)
	}
}

func TestBuildQuerySortOrder(t *testing.T) {
	params := QueryParams{
		Terms:     map[string]string{"title": "LLM"},
		Max:       50,
		SortBy:    "submitted",
		SortOrder: "desc",
	}
	u := BuildQueryURL(params)
	parsed, _ := url.Parse(u)
	q := parsed.Query()

	if q.Get("sortBy") != "submittedDate" {
		t.Errorf("sortBy = %q", q.Get("sortBy"))
	}
	if q.Get("sortOrder") != "descending" {
		t.Errorf("sortOrder = %q", q.Get("sortOrder"))
	}
}

// helper
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
