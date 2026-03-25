// internal/orchestrator/search.go
package orchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/host452b/arxs/internal/log"
	"github.com/host452b/arxs/internal/model"
	"github.com/host452b/arxs/internal/provider"
)

type providerResult struct {
	id     provider.ProviderID
	papers []model.Paper
	err    error
}

// Search fans out to all providers concurrently, deduplicates, and returns grouped results.
// providers must be in priority order (primary first).
func Search(
	ctx context.Context,
	providers []provider.Provider,
	q provider.Query,
	f provider.SubjectFilter,
	logger *log.Logger,
) (*model.MultiSourceResult, error) {
	start := time.Now()
	l := logger.WithTraceID()
	l.Info("subject_lookup", map[string]any{
		"providers": providerIDs(providers),
		"keywords":  q.Keywords,
	})

	ch := make(chan providerResult, len(providers))
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		go func(p provider.Provider) {
			defer wg.Done()
			pStart := time.Now()
			l.Info("provider_start", map[string]any{"provider": p.ID(), "max": q.Max})
			papers, err := p.Search(ctx, q, f)
			elapsed := time.Since(pStart).Milliseconds()
			if err != nil {
				l.Error("provider_error", map[string]any{"provider": p.ID(), "error": err.Error(), "elapsed_ms": elapsed})
			} else {
				l.Info("provider_done", map[string]any{"provider": p.ID(), "count": len(papers), "elapsed_ms": elapsed})
			}
			ch <- providerResult{id: p.ID(), papers: papers, err: err}
		}(p)
	}

	wg.Wait()
	close(ch)

	// Collect results in provider order
	byID := map[provider.ProviderID][]model.Paper{}
	var errs []error
	for r := range ch {
		if r.err != nil {
			errs = append(errs, r.err)
		} else {
			byID[r.id] = r.papers
		}
	}

	// If all failed, return error
	if len(errs) == len(providers) {
		return nil, errors.Join(errs...)
	}

	// Dedup: key = DOI (if present) or normalized title. Primary source wins.
	seen := map[string]bool{}
	var groups []model.SourceGroup
	total := 0

	for _, p := range providers {
		papers, ok := byID[p.ID()]
		if !ok {
			continue
		}
		var kept []model.Paper
		for _, paper := range papers {
			key := dedupKey(paper)
			if seen[key] {
				continue
			}
			seen[key] = true
			kept = append(kept, paper)
		}
		if len(kept) > 0 {
			groups = append(groups, model.SourceGroup{
				Source: string(p.ID()),
				Count:  len(kept),
				Papers: kept,
			})
			total += len(kept)
		}
	}

	l.Info("dedup", map[string]any{
		"before":     countAll(byID),
		"after":      total,
		"removed":    countAll(byID) - total,
		"elapsed_ms": time.Since(start).Milliseconds(),
	})
	l.Info("done", map[string]any{"total": total, "elapsed_ms": time.Since(start).Milliseconds()})

	return &model.MultiSourceResult{Groups: groups, Total: total}, nil
}

func dedupKey(p model.Paper) string {
	if p.DOI != "" {
		return "doi:" + strings.ToLower(p.DOI)
	}
	return "title:" + normalizeTitle(p.Title)
}

func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func providerIDs(providers []provider.Provider) []string {
	ids := make([]string, len(providers))
	for i, p := range providers {
		ids[i] = string(p.ID())
	}
	return ids
}

func countAll(byID map[provider.ProviderID][]model.Paper) int {
	n := 0
	for _, papers := range byID {
		n += len(papers)
	}
	return n
}
