// internal/subject/registry_test.go
package subject_test

import (
	"testing"
	"github.com/host452b/arxs/v2/internal/provider"
	"github.com/host452b/arxs/v2/internal/subject"
)

func TestLookup_CSAI(t *testing.T) {
	result, err := subject.Lookup([]string{"cs.AI"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Providers) == 0 {
		t.Fatal("expected at least one provider")
	}
	if result.Providers[0] != provider.ProviderArxiv {
		t.Errorf("expected arxiv as primary, got %s", result.Providers[0])
	}
	if len(result.Filter.ArxivCats) == 0 {
		t.Error("expected ArxivCats to be populated")
	}
}

func TestLookup_Unknown(t *testing.T) {
	_, err := subject.Lookup([]string{"astrophysics_typo"})
	if err == nil {
		t.Fatal("expected error for unknown subject")
	}
}

func TestLookup_Multiple(t *testing.T) {
	result, err := subject.Lookup([]string{"cs.AI", "sociology"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should contain both arxiv (from cs.AI) and socarxiv (from sociology)
	found := map[provider.ProviderID]bool{}
	for _, p := range result.Providers {
		found[p] = true
	}
	if !found[provider.ProviderArxiv] {
		t.Error("expected arxiv in providers")
	}
	if !found[provider.ProviderSocArxiv] {
		t.Error("expected socarxiv in providers")
	}
}

func TestLookup_CommaAlias(t *testing.T) {
	// "cs" top-level alias should resolve
	result, err := subject.Lookup([]string{"cs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Providers[0] != provider.ProviderArxiv {
		t.Errorf("expected arxiv as primary for cs, got %s", result.Providers[0])
	}
}

func TestLookup_EmptyInput(t *testing.T) {
	_, err := subject.Lookup([]string{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestLookup_CommaSeparated(t *testing.T) {
	result, err := subject.Lookup([]string{"cs.ai,sociology"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := map[provider.ProviderID]bool{}
	for _, p := range result.Providers {
		found[p] = true
	}
	if !found[provider.ProviderArxiv] {
		t.Error("expected arxiv in providers")
	}
	if !found[provider.ProviderSocArxiv] {
		t.Error("expected socarxiv in providers")
	}
}

func TestLookup_OSFProvidersMerged(t *testing.T) {
	// psychology → socarxiv, education → edarxiv; both must be present
	result, err := subject.Lookup([]string{"psychology", "education"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := map[string]bool{}
	for _, p := range result.Filter.OSFProviders {
		found[p] = true
	}
	if !found["socarxiv"] {
		t.Error("expected socarxiv in OSFProviders")
	}
	if !found["edarxiv"] {
		t.Error("expected edarxiv in OSFProviders")
	}
}
