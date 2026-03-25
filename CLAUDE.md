# arxs — Claude Code Project Guide

## What This Project Is
A Go CLI tool that searches academic papers across 5 sources:
arXiv, Zenodo, SocArXiv (OSF), EdArXiv (OSF), and OpenAlex.

## Architecture
- `internal/provider/` — one sub-package per source, all implement `Provider` interface
- `internal/subject/registry.go` — maps `-s` flags to provider lists + per-source filters
- `internal/orchestrator/` — concurrent fan-out, dedup by DOI/title, grouped results
- `internal/log/` — structured JSON logger (stderr, `--debug` or `ARXS_DEBUG=1`)
- `cmd/` — CLI commands (cobra); no business logic, only wiring

## Key Patterns
- Each provider has `WithBaseURL(url)` and `WithRateInterval(d)` options for testing
- All HTTP calls use `http.NewRequestWithContext` (context cancellation / timeout support)
- Mock HTTP servers via `httptest.NewServer` — no real API calls in tests
- `model.MultiSourceResult.AllPapers()` returns flat slice in display order

## Running Tests
```bash
go test ./... -race
```

## Adding a New Provider
1. Create `internal/provider/<name>/provider.go` implementing `Provider` interface
2. Add test file with 5 test cases (OK, HTTPError, Empty, Timeout, MalformedJSON)
3. Register in `internal/subject/registry.go` entries
4. Add to `buildProviders()` map in `cmd/search.go`

## Debug Logging
```bash
arxs search -k "test" -s cs.AI --debug 2>debug.log
# or
ARXS_DEBUG=1 arxs search -k "test" -s cs.AI 2>debug.log
```
