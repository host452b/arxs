// internal/orchestrator/search_test.go
package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/host452b/arxs/v2/internal/log"
	"github.com/host452b/arxs/v2/internal/model"
	"github.com/host452b/arxs/v2/internal/orchestrator"
	"github.com/host452b/arxs/v2/internal/provider"
)

// mockProvider is a controllable Provider for testing.
type mockProvider struct {
	id     provider.ProviderID
	papers []model.Paper
	err    error
}

func (m *mockProvider) ID() provider.ProviderID { return m.id }
func (m *mockProvider) Search(_ context.Context, _ provider.Query, _ provider.SubjectFilter) ([]model.Paper, error) {
	return m.papers, m.err
}
func (m *mockProvider) DownloadPDF(_ context.Context, _ model.Paper) ([]byte, error) {
	return nil, nil
}

func TestSearch_GroupsBySource(t *testing.T) {
	p1 := &mockProvider{id: "arxiv", papers: []model.Paper{{ID: "a1", Source: "arxiv", Title: "Paper A"}}}
	p2 := &mockProvider{id: "zenodo", papers: []model.Paper{{ID: "z1", Source: "zenodo", Title: "Paper Z"}}}

	result, err := orchestrator.Search(context.Background(), []provider.Provider{p1, p2},
		provider.Query{Keywords: "test", Max: 10}, provider.SubjectFilter{}, log.New(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total: got %d want 2", result.Total)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("groups: got %d want 2", len(result.Groups))
	}
	if result.Groups[0].Source != "arxiv" {
		t.Errorf("first group source: got %s want arxiv", result.Groups[0].Source)
	}
}

func TestSearch_DeduplicatesByDOI(t *testing.T) {
	paper := model.Paper{ID: "a1", DOI: "10.1234/test", Source: "arxiv", Title: "Shared"}
	duplicate := model.Paper{ID: "z1", DOI: "10.1234/test", Source: "zenodo", Title: "Shared"}

	p1 := &mockProvider{id: "arxiv", papers: []model.Paper{paper}}
	p2 := &mockProvider{id: "zenodo", papers: []model.Paper{duplicate}}

	result, err := orchestrator.Search(context.Background(), []provider.Provider{p1, p2},
		provider.Query{Keywords: "test", Max: 10}, provider.SubjectFilter{}, log.New(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected dedup to 1, got %d", result.Total)
	}
	// Primary (arxiv) should be kept
	if result.Groups[0].Papers[0].Source != "arxiv" {
		t.Error("primary source should be kept after dedup")
	}
}

func TestSearch_DeduplicatesByTitle(t *testing.T) {
	// Same title, no DOI — should dedup by normalized title
	paper1 := model.Paper{ID: "a1", Source: "arxiv", Title: "Machine Learning Survey"}
	paper2 := model.Paper{ID: "z1", Source: "zenodo", Title: "Machine Learning Survey"}

	p1 := &mockProvider{id: "arxiv", papers: []model.Paper{paper1}}
	p2 := &mockProvider{id: "zenodo", papers: []model.Paper{paper2}}

	result, err := orchestrator.Search(context.Background(), []provider.Provider{p1, p2},
		provider.Query{Keywords: "test", Max: 10}, provider.SubjectFilter{}, log.New(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected title dedup to 1, got %d", result.Total)
	}
	if result.Groups[0].Papers[0].Source != "arxiv" {
		t.Error("primary source (arxiv) should be kept after title dedup")
	}
}

func TestSearch_PartialFailure(t *testing.T) {
	p1 := &mockProvider{id: "arxiv", papers: []model.Paper{{ID: "a1", Source: "arxiv"}}}
	p2 := &mockProvider{id: "zenodo", err: errors.New("network error")}

	result, err := orchestrator.Search(context.Background(), []provider.Provider{p1, p2},
		provider.Query{Keywords: "test", Max: 10}, provider.SubjectFilter{}, log.New(false))
	if err != nil {
		t.Fatalf("partial failure should not return error when at least one succeeds: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 paper from successful provider, got %d", result.Total)
	}
}

func TestSearch_AllFail(t *testing.T) {
	p1 := &mockProvider{id: "arxiv", err: errors.New("error 1")}
	p2 := &mockProvider{id: "zenodo", err: errors.New("error 2")}

	_, err := orchestrator.Search(context.Background(), []provider.Provider{p1, p2},
		provider.Query{Keywords: "test", Max: 10}, provider.SubjectFilter{}, log.New(false))
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}
