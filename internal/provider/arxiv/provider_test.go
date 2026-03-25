// internal/provider/arxiv/provider_test.go
package arxiv_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/host452b/arxs/internal/model"
	"github.com/host452b/arxs/internal/provider"
	arxivprovider "github.com/host452b/arxs/internal/provider/arxiv"
)

func sampleAtomXML() []byte {
	data, _ := os.ReadFile("../../../testdata/sample_response.xml")
	return data
}

func TestArxivProvider_Search_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write(sampleAtomXML())
	}))
	defer srv.Close()

	p := arxivprovider.New(arxivprovider.WithBaseURL(srv.URL))
	q := provider.Query{Keywords: "transformer", Max: 5}
	f := provider.SubjectFilter{ArxivCats: []string{"cs.AI"}}

	papers, err := p.Search(context.Background(), q, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) == 0 {
		t.Fatal("expected papers, got none")
	}
	if papers[0].Source != "arxiv" {
		t.Errorf("expected source=arxiv, got %s", papers[0].Source)
	}
}

func TestArxivProvider_Search_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := arxivprovider.New(arxivprovider.WithBaseURL(srv.URL))
	_, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestArxivProvider_Search_Empty(t *testing.T) {
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <opensearch:totalResults xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">0</opensearch:totalResults>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(emptyXML))
	}))
	defer srv.Close()

	p := arxivprovider.New(arxivprovider.WithBaseURL(srv.URL))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 0 {
		t.Errorf("expected empty, got %d papers", len(papers))
	}
}

func TestArxivProvider_Search_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// hang until client cancels
		<-r.Context().Done()
	}))
	defer srv.Close()

	p := arxivprovider.New(arxivprovider.WithBaseURL(srv.URL), arxivprovider.WithRateInterval(0))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := p.Search(ctx, provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestArxivProvider_DownloadPDF_NoURL(t *testing.T) {
	p := arxivprovider.New()
	_, err := p.DownloadPDF(context.Background(), model.Paper{ID: "test", PDFUrl: ""})
	if err == nil {
		t.Fatal("expected error for empty PDFUrl")
	}
}

func TestArxivProvider_DownloadPDF_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("PDF content"))
	}))
	defer srv.Close()

	p := arxivprovider.New(arxivprovider.WithBaseURL(srv.URL), arxivprovider.WithRateInterval(0))
	data, err := p.DownloadPDF(context.Background(), model.Paper{ID: "test", PDFUrl: srv.URL + "/pdf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "PDF content" {
		t.Errorf("unexpected data: %s", data)
	}
}
