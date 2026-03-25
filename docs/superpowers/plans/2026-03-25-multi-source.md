# Multi-Source Paper Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor arxs from arXiv-only to a 5-source aggregator (arXiv, Zenodo, SocArXiv, EdArXiv, OpenAlex) with subject-driven source routing, grouped output, and structured AI logging.

**Architecture:** Provider interface pattern — each source implements `Provider`; a `SubjectRegistry` maps `-s` flags to provider lists + per-source subject filters; the `Orchestrator` fans out concurrently, deduplicates, and groups results. Existing `internal/api/` is preserved and wrapped by the arXiv provider.

**Tech Stack:** Go stdlib only (no new external deps). `net/http`, `encoding/json`, `encoding/xml`, `context`, `crypto/rand`, `errors` (Go 1.20+ `errors.Join`), `httptest` for tests.

**Spec:** `docs/superpowers/specs/2026-03-25-multi-source-design.md`

---

## File Map

**New files:**
```
internal/log/log.go
internal/provider/provider.go
internal/provider/arxiv/provider.go
internal/provider/arxiv/provider_test.go
internal/provider/zenodo/provider.go
internal/provider/zenodo/provider_test.go
internal/provider/socarxiv/provider.go
internal/provider/socarxiv/provider_test.go
internal/provider/edarxiv/provider.go
internal/provider/edarxiv/provider_test.go
internal/provider/openalex/provider.go
internal/provider/openalex/provider_test.go
internal/subject/registry.go
internal/subject/registry_test.go
internal/orchestrator/search.go
internal/orchestrator/search_test.go
```

**Modified files:**
```
internal/model/paper.go          — add Source, SourceURL to Paper; add MultiSourceResult/SourceGroup
internal/cache/cache.go          — add GetMulti/SetMulti for MultiSourceResult
internal/store/store.go          — add WriteMultiSourceResult/ReadMultiSourceResult
cmd/root.go                      — add --debug persistent flag + PersistentPreRunE
cmd/search.go                    — -s StringArray, orchestrator wiring, grouped output
cmd/download.go                  — read MultiSourceResult, dispatch DownloadPDF by source
```

---

## Task 1: Extend model

**Files:**
- Modify: `internal/model/paper.go`

- [ ] **Step 1: Add Source/SourceURL fields to Paper and new types**

```go
// internal/model/paper.go
package model

// Paper represents a research paper from any supported source.
type Paper struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Authors    []string `json:"authors"`
	Abstract   string   `json:"abstract"`
	Categories []string `json:"categories"`
	Published  string   `json:"published"`
	Updated    string   `json:"updated"`
	PDFUrl     string   `json:"pdf_url"`
	HTMLUrl    string   `json:"html_url"`
	AbsUrl     string   `json:"abs_url"`
	Citations  int      `json:"citations"`
	DOI        string   `json:"doi,omitempty"`
	Source     string   `json:"source"`      // "arxiv"|"zenodo"|"socarxiv"|"edarxiv"|"openalex"
	SourceURL  string   `json:"source_url"`  // canonical page URL on the source platform
}

// SearchResult is kept for backward compatibility (arXiv-only single-source).
type SearchResult struct {
	Query        QueryMeta `json:"query"`
	TotalResults int       `json:"total_results"`
	ReturnCount  int       `json:"return_count"`
	Papers       []Paper   `json:"papers"`
}

// MultiSourceResult is the top-level output for multi-source searches.
type MultiSourceResult struct {
	Query  QueryMeta   `json:"query"`
	Groups []SourceGroup `json:"groups"`
	Total  int         `json:"total"`
}

// SourceGroup holds results from one provider.
type SourceGroup struct {
	Source string  `json:"source"`
	Count  int     `json:"count"`
	Papers []Paper `json:"papers"`
}

// QueryMeta records the search parameters used.
type QueryMeta struct {
	Terms      map[string]string `json:"terms"`
	Subjects   []string          `json:"subjects"`
	Op         string            `json:"op"`
	From       string            `json:"from"`
	To         string            `json:"to"`
	Max        int               `json:"max"`
	SearchedAt string            `json:"searched_at"`
}

// AllPapers returns a flat slice of all papers across all groups, in display order.
func (r *MultiSourceResult) AllPapers() []Paper {
	var out []Paper
	for _, g := range r.Groups {
		out = append(out, g.Papers...)
	}
	return out
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /localhome/swqa/workspace/axgs/arxs && go build ./internal/model/...
```
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/model/paper.go
git commit -m "feat(model): add Source/DOI fields to Paper, add MultiSourceResult"
```

---

## Task 2: Add log package

**Files:**
- Create: `internal/log/log.go`

- [ ] **Step 1: Create the logger**

```go
// internal/log/log.go
package log

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type contextKey struct{}

// Logger writes structured JSON log lines to stderr when enabled.
type Logger struct {
	enabled bool
	traceID string
}

// New creates a Logger. If enabled is false, all methods are no-ops.
func New(enabled bool) *Logger {
	return &Logger{enabled: enabled}
}

// WithTraceID returns a new Logger with a freshly generated trace_id.
func (l *Logger) WithTraceID() *Logger {
	if !l.enabled {
		return l
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return &Logger{enabled: true, traceID: fmt.Sprintf("%x", b)}
}

// Info logs an info-level event.
func (l *Logger) Info(step string, fields map[string]any) {
	l.write("info", step, fields)
}

// Error logs an error-level event.
func (l *Logger) Error(step string, fields map[string]any) {
	l.write("error", step, fields)
}

func (l *Logger) write(level, step string, fields map[string]any) {
	if !l.enabled {
		return
	}
	entry := map[string]any{
		"ts":       time.Now().UTC().Format(time.RFC3339),
		"level":    level,
		"step":     step,
		"trace_id": l.traceID,
	}
	for k, v := range fields {
		entry[k] = v
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "%s\n", data)
}

// WithLogger stores a Logger in the context.
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the Logger from context.
// Returns a no-op logger if none is present.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(contextKey{}).(*Logger); ok {
		return l
	}
	return &Logger{enabled: false}
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./internal/log/...
```
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/log/log.go
git commit -m "feat(log): add structured JSON logger with trace_id and context injection"
```

---

## Task 3: Define Provider interface

**Files:**
- Create: `internal/provider/provider.go`

- [ ] **Step 1: Write provider.go**

```go
// internal/provider/provider.go
package provider

import "context"
import "github.com/joejiang/arxs/internal/model"

// ProviderID identifies a paper source.
type ProviderID string

const (
	ProviderArxiv    ProviderID = "arxiv"
	ProviderZenodo   ProviderID = "zenodo"
	ProviderSocArxiv ProviderID = "socarxiv"
	ProviderEdArxiv  ProviderID = "edarxiv"
	ProviderOpenAlex ProviderID = "openalex"
)

// Query carries search parameters for all providers.
// Terms/Op are arXiv-specific; Keywords is a pre-built string for other providers.
type Query struct {
	Terms     map[string]string // arXiv field-specific: "title"→expr, "abs"→expr, etc.
	Op        string            // "and"|"or" between Terms (arXiv only)
	Keywords  string            // flattened keyword string for non-arXiv sources
	From      string            // YYYY-MM-DD
	To        string            // YYYY-MM-DD
	Max       int
	SortBy    string
	SortOrder string
}

// SubjectFilter carries per-source subject filter parameters.
type SubjectFilter struct {
	ArxivCats        []string // arXiv: ["cs.AI","cs.LG"]
	OpenAlexConcepts []string // OpenAlex concept IDs: ["C41008148"]
	ZenodoKeywords   []string // Zenodo subject keywords: ["machine learning"]
	OSFProvider      string   // "socarxiv" or "edarxiv"
	OSFSubjects      []string // OSF subject display strings
}

// Provider is the interface all paper sources must implement.
type Provider interface {
	ID() ProviderID
	Search(ctx context.Context, q Query, f SubjectFilter) ([]model.Paper, error)
	DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error)
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./internal/provider/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/provider/provider.go
git commit -m "feat(provider): define Provider interface, Query, and SubjectFilter types"
```

---

## Task 4: Subject Registry

**Files:**
- Create: `internal/subject/registry.go`
- Create: `internal/subject/registry_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/subject/registry_test.go
package subject_test

import (
	"testing"
	"github.com/joejiang/arxs/internal/provider"
	"github.com/joejiang/arxs/internal/subject"
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
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/subject/... 2>&1 | head -5
```
Expected: `cannot find package` or compilation error.

- [ ] **Step 3: Implement registry.go**

```go
// internal/subject/registry.go
package subject

import (
	"fmt"
	"strings"

	"github.com/joejiang/arxs/internal/provider"
)

// LookupResult holds the ordered provider list and merged filters for a set of subjects.
type LookupResult struct {
	Providers []provider.ProviderID
	Filter    provider.SubjectFilter
}

// entry defines mapping for one subject code or alias.
type entry struct {
	providers        []provider.ProviderID
	arxivCats        []string
	openAlexConcepts []string
	zenodoKeywords   []string
	osfProvider      string
	osfSubjects      []string
}

// registry maps subject codes and aliases to their entry.
var registry = map[string]entry{
	// ── Computer Science ──────────────────────────────────────────────
	"cs":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs"}, openAlexConcepts: []string{"C41008148"}, zenodoKeywords: []string{"computer science"}},
	"cs.ai": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.AI"}, openAlexConcepts: []string{"C154945302"}, zenodoKeywords: []string{"artificial intelligence"}},
	"cs.lg": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.LG"}, openAlexConcepts: []string{"C119857082"}, zenodoKeywords: []string{"machine learning"}},
	"cs.cl": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.CL"}, openAlexConcepts: []string{"C204321447"}, zenodoKeywords: []string{"natural language processing"}},
	"cs.cv": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.CV"}, openAlexConcepts: []string{"C31972630"}, zenodoKeywords: []string{"computer vision"}},
	"cs.cr": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.CR"}, openAlexConcepts: []string{"C38652104"}, zenodoKeywords: []string{"cybersecurity"}},
	"cs.ro": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.RO"}, openAlexConcepts: []string{"C11413529"}, zenodoKeywords: []string{"robotics"}},
	"cs.cy": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderSocArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.CY"}, openAlexConcepts: []string{"C17744445"}, zenodoKeywords: []string{"computers and society"}, osfProvider: "socarxiv", osfSubjects: []string{"Social and Behavioral Sciences"}},
	"cs.hc": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.HC"}, openAlexConcepts: []string{"C121332964"}, zenodoKeywords: []string{"human-computer interaction"}},
	"cs.se": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.SE"}, zenodoKeywords: []string{"software engineering"}},
	"cs.dc": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.DC"}, zenodoKeywords: []string{"distributed computing"}},
	"cs.ni": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.NI"}, zenodoKeywords: []string{"computer networking"}},
	"cs.gt": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"cs.GT"}, openAlexConcepts: []string{"C2993651"}, zenodoKeywords: []string{"game theory"}},
	"cs.ir": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.IR"}, zenodoKeywords: []string{"information retrieval"}},

	// ── Physics ───────────────────────────────────────────────────────
	"physics":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"physics", "astro-ph", "cond-mat", "gr-qc", "hep-ex", "hep-lat", "hep-ph", "hep-th", "math-ph", "nlin", "nucl-ex", "nucl-th", "quant-ph"}, zenodoKeywords: []string{"physics"}},
	"hep-th":     {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"hep-th"}, openAlexConcepts: []string{"C121332964"}, zenodoKeywords: []string{"high energy physics theoretical"}},
	"hep-ex":     {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"hep-ex"}, zenodoKeywords: []string{"high energy physics experimental"}},
	"hep-ph":     {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"hep-ph"}, zenodoKeywords: []string{"phenomenology"}},
	"hep-lat":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"hep-lat"}, zenodoKeywords: []string{"lattice QCD"}},
	"quant-ph":   {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"quant-ph"}, openAlexConcepts: []string{"C62520636"}, zenodoKeywords: []string{"quantum computing"}},
	"cond-mat":   {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cond-mat"}, zenodoKeywords: []string{"condensed matter"}},
	"astro-ph":   {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"astro-ph"}, zenodoKeywords: []string{"astrophysics"}},
	"gr-qc":      {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"gr-qc"}, zenodoKeywords: []string{"general relativity"}},
	"nucl-ex":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"nucl-ex"}, zenodoKeywords: []string{"nuclear physics"}},
	"nucl-th":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"nucl-th"}, zenodoKeywords: []string{"nuclear theory"}},
	"math-ph":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math-ph"}, zenodoKeywords: []string{"mathematical physics"}},
	"nlin":       {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"nlin"}, zenodoKeywords: []string{"nonlinear science"}},

	// ── Mathematics ───────────────────────────────────────────────────
	"math":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math"}, openAlexConcepts: []string{"C33923547"}, zenodoKeywords: []string{"mathematics"}},
	"math.ag": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.AG"}, zenodoKeywords: []string{"algebraic geometry"}},
	"math.nt": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.NT"}, zenodoKeywords: []string{"number theory"}},
	"math.pr": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.PR"}, zenodoKeywords: []string{"probability"}},
	"math.co": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.CO"}, zenodoKeywords: []string{"combinatorics"}},
	"math.na": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.NA"}, zenodoKeywords: []string{"numerical analysis"}},
	"math.lo": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"math.LO"}, openAlexConcepts: []string{"C138885662"}, zenodoKeywords: []string{"mathematical logic"}},

	// ── Statistics ────────────────────────────────────────────────────
	"stat":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"stat"}, openAlexConcepts: []string{"C161191863"}, zenodoKeywords: []string{"statistics"}},
	"stat.ml": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"stat.ML"}, zenodoKeywords: []string{"statistical machine learning"}},

	// ── Quantitative Finance & Economics ──────────────────────────────
	"q-fin":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"q-fin"}, openAlexConcepts: []string{"C187279774"}, zenodoKeywords: []string{"quantitative finance"}},
	"q-fin.tr": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"q-fin.TR"}, zenodoKeywords: []string{"algorithmic trading"}},
	"q-fin.rm": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"q-fin.RM"}, zenodoKeywords: []string{"risk management"}},
	"econ":     {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"econ"}, openAlexConcepts: []string{"C162324750"}, zenodoKeywords: []string{"economics"}},
	"econ.em":  {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"econ.EM"}, zenodoKeywords: []string{"econometrics"}},

	// ── Quantitative Biology & EESS ───────────────────────────────────
	"q-bio":  {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"q-bio"}, openAlexConcepts: []string{"C86803240"}, zenodoKeywords: []string{"quantitative biology"}},
	"eess":   {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"eess"}, openAlexConcepts: []string{"C41008148"}, zenodoKeywords: []string{"electrical engineering"}},
	"eess.sp": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"eess.SP"}, zenodoKeywords: []string{"signal processing"}},

	// ── Social Sciences ───────────────────────────────────────────────
	"sociology":   {providers: []provider.ProviderID{provider.ProviderSocArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, openAlexConcepts: []string{"C144024400"}, zenodoKeywords: []string{"sociology"}, osfProvider: "socarxiv", osfSubjects: []string{"Social and Behavioral Sciences"}},
	"law":         {providers: []provider.ProviderID{provider.ProviderSocArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, openAlexConcepts: []string{"C18214049"}, zenodoKeywords: []string{"law"}, osfProvider: "socarxiv", osfSubjects: []string{"Law"}},
	"psychology":  {providers: []provider.ProviderID{provider.ProviderSocArxiv, provider.ProviderEdArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C15744967"}, zenodoKeywords: []string{"psychology"}, osfProvider: "socarxiv", osfSubjects: []string{"Social and Behavioral Sciences"}},
	"political":   {providers: []provider.ProviderID{provider.ProviderSocArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, openAlexConcepts: []string{"C17744445"}, zenodoKeywords: []string{"political science"}, osfProvider: "socarxiv", osfSubjects: []string{"Political Science"}},
	"economics":   {providers: []provider.ProviderID{provider.ProviderOpenAlex, provider.ProviderArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C162324750"}, zenodoKeywords: []string{"economics"}},
	"management":  {providers: []provider.ProviderID{provider.ProviderOpenAlex, provider.ProviderSocArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C144133560"}, zenodoKeywords: []string{"management"}},

	// ── Education ─────────────────────────────────────────────────────
	"education": {providers: []provider.ProviderID{provider.ProviderEdArxiv, provider.ProviderSocArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C142362112"}, zenodoKeywords: []string{"education"}, osfProvider: "edarxiv", osfSubjects: []string{"Education"}},

	// ── Philosophy ────────────────────────────────────────────────────
	"philosophy": {providers: []provider.ProviderID{provider.ProviderOpenAlex, provider.ProviderSocArxiv, provider.ProviderZenodo}, arxivCats: []string{"physics.hist-ph"}, openAlexConcepts: []string{"C138885662"}, zenodoKeywords: []string{"philosophy"}, osfProvider: "socarxiv", osfSubjects: []string{"Philosophy"}},
	"ethics":     {providers: []provider.ProviderID{provider.ProviderOpenAlex, provider.ProviderSocArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C119599485"}, zenodoKeywords: []string{"ethics"}},
}

// ErrUnknownSubject is returned when a subject code or alias is not in the registry.
type ErrUnknownSubject struct {
	Subject string
}

func (e ErrUnknownSubject) Error() string {
	return fmt.Sprintf("unknown subject %q — run 'arxs search --list-subjects' for valid values", e.Subject)
}

// Lookup resolves a list of subject strings into a merged LookupResult.
// Each string is lowercased before lookup. Comma-separated entries are split.
// Returns ErrUnknownSubject if any entry is not found.
func Lookup(subjects []string) (*LookupResult, error) {
	// Expand comma-separated entries
	var expanded []string
	for _, s := range subjects {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part != "" {
				expanded = append(expanded, part)
			}
		}
	}

	// Validate all subjects first
	for _, s := range expanded {
		if _, ok := registry[s]; !ok {
			return nil, ErrUnknownSubject{Subject: s}
		}
	}

	// Merge entries
	providerSeen := map[provider.ProviderID]bool{}
	var providers []provider.ProviderID
	filter := provider.SubjectFilter{}

	for _, s := range expanded {
		e := registry[s]
		for _, p := range e.providers {
			if !providerSeen[p] {
				providerSeen[p] = true
				providers = append(providers, p)
			}
		}
		filter.ArxivCats = unionStrings(filter.ArxivCats, e.arxivCats)
		filter.OpenAlexConcepts = unionStrings(filter.OpenAlexConcepts, e.openAlexConcepts)
		filter.ZenodoKeywords = unionStrings(filter.ZenodoKeywords, e.zenodoKeywords)
		filter.OSFSubjects = unionStrings(filter.OSFSubjects, e.osfSubjects)
		if filter.OSFProvider == "" && e.osfProvider != "" {
			filter.OSFProvider = e.osfProvider
		}
	}

	return &LookupResult{Providers: providers, Filter: filter}, nil
}

// ValidSubjects returns a sorted list of all known subject codes and aliases.
func ValidSubjects() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	for _, s := range a {
		seen[s] = true
	}
	result := append([]string{}, a...)
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/subject/... -v
```
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/subject/
git commit -m "feat(subject): add subject registry with 50+ arXiv codes and discipline aliases"
```

---

## Task 5: arXiv Provider

**Files:**
- Create: `internal/provider/arxiv/provider.go`
- Create: `internal/provider/arxiv/provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/provider/arxiv/provider_test.go
package arxiv_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/joejiang/arxs/internal/provider"
	arxivprovider "github.com/joejiang/arxs/internal/provider/arxiv"
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
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./internal/provider/arxiv/... 2>&1 | head -5
```
Expected: compilation error (package doesn't exist yet).

- [ ] **Step 3: Export UserAgent from internal/api/client.go FIRST**

In `internal/api/client.go`, change:
```go
// before
const userAgent = "arxs/1.0 ..."

// after — exported so provider packages can use it
const UserAgent = "arxs/1.0 ..."
```
Update all usages of `userAgent` inside `client.go` and `citations.go` to `UserAgent`. Compile check:
```bash
go build ./internal/api/...
```
Expected: no errors.

- [ ] **Step 4: Implement arxiv provider**

```go
// internal/provider/arxiv/provider.go
package arxiv

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/joejiang/arxs/internal/api"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/provider"
)

const defaultBaseURL = "https://export.arxiv.org/api/query"

// Option configures the arXiv provider.
type Option func(*Provider)

// WithBaseURL overrides the API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(p *Provider) { p.baseURL = url }
}

// WithRateInterval overrides the rate limiter interval (for testing).
func WithRateInterval(d time.Duration) Option {
	return func(p *Provider) { p.rateLimiter = api.NewRateLimiter(d) }
}

// Provider implements provider.Provider for arXiv.
type Provider struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *api.RateLimiter
}

// New creates an arXiv Provider.
func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: api.NewRateLimiter(3 * time.Second),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) ID() provider.ProviderID { return provider.ProviderArxiv }

func (p *Provider) Search(ctx context.Context, q provider.Query, f provider.SubjectFilter) ([]model.Paper, error) {
	p.rateLimiter.Wait()

	// Build arXiv QueryParams from provider.Query + SubjectFilter
	params := api.QueryParams{
		Terms:     q.Terms,
		Subjects:  f.ArxivCats,
		Op:        q.Op,
		From:      q.From,
		To:        q.To,
		Max:       q.Max,
		SortBy:    q.SortBy,
		SortOrder: q.SortOrder,
	}
	// Fallback: if no Terms, use Keywords as all-field search
	if len(params.Terms) == 0 && q.Keywords != "" {
		params.Terms = map[string]string{"all": q.Keywords}
	}

	queryURL := api.BuildQueryURL(params)
	// Replace host if baseURL is overridden for testing
	if p.baseURL != defaultBaseURL {
		rest := strings.SplitN(queryURL, "?", 2)
		if len(rest) == 2 {
			queryURL = p.baseURL + "?" + rest[1]
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("arxiv: creating request: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("arxiv: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("arxiv: reading response: %w", err)
	}

	feed, err := model.ParseAtomFeed(body)
	if err != nil {
		return nil, fmt.Errorf("arxiv: parsing XML: %w", err)
	}
	if apiErr := feed.APIError(); apiErr != "" {
		return nil, fmt.Errorf("arxiv: API error: %s", apiErr)
	}

	papers := make([]model.Paper, len(feed.Entries))
	for i, e := range feed.Entries {
		papers[i] = e.ToPaper()
		papers[i].Source = "arxiv"
		papers[i].SourceURL = papers[i].AbsUrl
	}
	return papers, nil
}

func (p *Provider) DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error) {
	if paper.PDFUrl == "" {
		return nil, fmt.Errorf("arxiv: no PDF URL for %s", paper.ID)
	}
	p.rateLimiter.Wait()
	req, err := http.NewRequestWithContext(ctx, "GET", paper.PDFUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", api.UserAgent)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv: download HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/provider/arxiv/... -v -race
```
Expected: 4 tests PASS.

- [ ] **Step 6: Confirm existing tests still pass**

```bash
go test ./... 2>&1 | grep -E "FAIL|ok"
```
Expected: all `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/provider/arxiv/ internal/api/client.go internal/api/citations.go
git commit -m "feat(provider/arxiv): wrap existing arXiv API as Provider interface"
```

---

## Task 6: Extend store

**Files:**
- Modify: `internal/store/store.go`

- [ ] **Step 1: Add MultiSourceResult read/write**

Append to `internal/store/store.go`:
```go
// WriteMultiSourceResult writes a MultiSourceResult to a JSON file.
func WriteMultiSourceResult(path string, result *model.MultiSourceResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ReadMultiSourceResult reads a MultiSourceResult from a JSON file.
func ReadMultiSourceResult(path string) (*model.MultiSourceResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result model.MultiSourceResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
```

- [ ] **Step 2: Add store test**

In `internal/store/store_test.go`, add:
```go
func TestWriteReadMultiSourceResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.json")

	want := &model.MultiSourceResult{
		Total: 2,
		Groups: []model.SourceGroup{
			{Source: "arxiv", Count: 1, Papers: []model.Paper{{ID: "2401.001", Source: "arxiv", Title: "T1"}}},
			{Source: "zenodo", Count: 1, Papers: []model.Paper{{ID: "z123", Source: "zenodo", Title: "T2"}}},
		},
	}
	if err := store.WriteMultiSourceResult(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadMultiSourceResult(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != want.Total {
		t.Errorf("total: got %d want %d", got.Total, want.Total)
	}
	if len(got.Groups) != 2 {
		t.Fatalf("groups: got %d want 2", len(got.Groups))
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/store/... -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/store/
git commit -m "feat(store): add WriteMultiSourceResult/ReadMultiSourceResult"
```

---

## Task 7: Extend cache

**Files:**
- Modify: `internal/cache/cache.go`

- [ ] **Step 1: Add GetMulti/SetMulti**

Append to `internal/cache/cache.go`:
```go
type multiCacheEntry struct {
	Date   string                  `json:"date"`
	Result model.MultiSourceResult `json:"result"`
}

// GetMulti retrieves a cached MultiSourceResult.
func (c *Cache) GetMulti(key string) (*model.MultiSourceResult, bool) {
	if c == nil {
		return nil, false
	}
	path := c.path("multi_" + key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entry multiCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}
	if entry.Date != today() {
		return nil, false
	}
	return &entry.Result, true
}

// SetMulti caches a MultiSourceResult.
func (c *Cache) SetMulti(key string, result *model.MultiSourceResult) error {
	if c == nil {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return err
	}
	entry := multiCacheEntry{Date: today(), Result: *result}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path("multi_"+key), data, 0644)
}
```

- [ ] **Step 2: Run existing cache tests**

```bash
go test ./internal/cache/... -v
```
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/cache/cache.go
git commit -m "feat(cache): add GetMulti/SetMulti for MultiSourceResult"
```

---

## Task 8: Zenodo Provider

**Files:**
- Create: `internal/provider/zenodo/provider.go`
- Create: `internal/provider/zenodo/provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/provider/zenodo/provider_test.go
package zenodo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joejiang/arxs/internal/provider"
	zenodoprovider "github.com/joejiang/arxs/internal/provider/zenodo"
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
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./internal/provider/zenodo/... 2>&1 | head -3
```

- [ ] **Step 3: Implement zenodo provider**

```go
// internal/provider/zenodo/provider.go
package zenodo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/joejiang/arxs/internal/api"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/provider"
)

const defaultBaseURL = "https://zenodo.org/api"

type Option func(*Provider)

func WithBaseURL(u string) Option    { return func(p *Provider) { p.baseURL = u } }
func WithRateInterval(d time.Duration) Option {
	return func(p *Provider) { p.rateLimiter = api.NewRateLimiter(d) }
}

type Provider struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *api.RateLimiter
}

func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: api.NewRateLimiter(1 * time.Second),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) ID() provider.ProviderID { return provider.ProviderZenodo }

// zenodoResponse mirrors the Zenodo search API response.
type zenodoResponse struct {
	Hits struct {
		Total int `json:"total"`
		Hits  []struct {
			ID       int    `json:"id"`
			DOI      string `json:"doi"`
			Metadata struct {
				Title           string `json:"title"`
				Description     string `json:"description"`
				PublicationDate string `json:"publication_date"`
				Creators        []struct {
					Name string `json:"name"`
				} `json:"creators"`
			} `json:"metadata"`
			Links struct {
				HTML string `json:"html"`
			} `json:"links"`
			Files []struct {
				Key   string `json:"key"`
				Links struct {
					Self string `json:"self"`
				} `json:"links"`
			} `json:"files"`
		} `json:"hits"`
	} `json:"hits"`
}

func (p *Provider) Search(ctx context.Context, q provider.Query, f provider.SubjectFilter) ([]model.Paper, error) {
	p.rateLimiter.Wait()

	// Build keyword query: combine provider keywords and user query
	kw := q.Keywords
	if len(f.ZenodoKeywords) > 0 {
		kw = strings.Join(f.ZenodoKeywords, " OR ") + " AND " + kw
	}

	params := url.Values{}
	params.Set("q", kw)
	params.Set("type", "publication")
	params.Set("size", fmt.Sprintf("%d", q.Max))
	if q.From != "" {
		params.Set("publication_date", q.From+"/"+q.To)
	}

	reqURL := p.baseURL + "/records?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("zenodo: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zenodo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zenodo: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("zenodo: reading response: %w", err)
	}

	var zr zenodoResponse
	if err := json.Unmarshal(body, &zr); err != nil {
		return nil, fmt.Errorf("zenodo: parsing JSON: %w", err)
	}

	papers := make([]model.Paper, 0, len(zr.Hits.Hits))
	for _, h := range zr.Hits.Hits {
		authors := make([]string, len(h.Metadata.Creators))
		for i, c := range h.Metadata.Creators {
			authors[i] = c.Name
		}
		// Find first PDF file
		pdfURL := ""
		for _, f := range h.Files {
			if strings.HasSuffix(strings.ToLower(f.Key), ".pdf") {
				pdfURL = f.Links.Self
				break
			}
		}
		papers = append(papers, model.Paper{
			ID:        fmt.Sprintf("zenodo.%d", h.ID),
			Title:     h.Metadata.Title,
			Authors:   authors,
			Abstract:  h.Metadata.Description,
			Published: h.Metadata.PublicationDate,
			DOI:       h.DOI,
			PDFUrl:    pdfURL,
			AbsUrl:    h.Links.HTML,
			Source:    "zenodo",
			SourceURL: h.Links.HTML,
		})
	}
	return papers, nil
}

func (p *Provider) DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error) {
	if paper.PDFUrl == "" {
		return nil, fmt.Errorf("zenodo: no PDF URL for %s", paper.ID)
	}
	p.rateLimiter.Wait()
	req, err := http.NewRequestWithContext(ctx, "GET", paper.PDFUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", api.UserAgent)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zenodo: download HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/provider/zenodo/... -v -race
```
Expected: 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/zenodo/
git commit -m "feat(provider/zenodo): implement Zenodo REST API provider"
```

---

## Task 9: SocArXiv Provider

**Files:**
- Create: `internal/provider/socarxiv/provider.go`
- Create: `internal/provider/socarxiv/provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/provider/socarxiv/provider_test.go
package socarxiv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joejiang/arxs/internal/provider"
	socarxivprovider "github.com/joejiang/arxs/internal/provider/socarxiv"
)

func sampleOSFResponse(providerID string) []byte {
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
					"html": "https://osf.io/preprints/" + providerID + "/abc12",
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
		w.Write(sampleOSFResponse("socarxiv"))
	}))
	defer srv.Close()

	p := socarxivprovider.New(socarxivprovider.WithBaseURL(srv.URL), socarxivprovider.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "inequality", Max: 5}, provider.SubjectFilter{OSFProvider: "socarxiv"})
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

func TestSocArxivProvider_Search_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", 403)
	}))
	defer srv.Close()

	p := socarxivprovider.New(socarxivprovider.WithBaseURL(srv.URL), socarxivprovider.WithRateInterval(0))
	_, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{OSFProvider: "socarxiv"})
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
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{OSFProvider: "socarxiv"})
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
	_, err := p.Search(ctx, provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{OSFProvider: "socarxiv"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
```

- [ ] **Step 2: Implement socarxiv provider**

```go
// internal/provider/socarxiv/provider.go
package socarxiv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/joejiang/arxs/internal/api"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/provider"
)

const defaultBaseURL = "https://api.osf.io/v2"
const providerID = "socarxiv"

type Option func(*Provider)

func WithBaseURL(u string) Option         { return func(p *Provider) { p.baseURL = u } }
func WithRateInterval(d time.Duration) Option {
	return func(p *Provider) { p.rateLimiter = api.NewRateLimiter(d) }
}

type Provider struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *api.RateLimiter
}

func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: api.NewRateLimiter(1 * time.Second),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) ID() provider.ProviderID { return provider.ProviderSocArxiv }

type osfResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			DatePublished string `json:"date_published"`
			DOI         string `json:"doi"`
		} `json:"attributes"`
		Links struct {
			HTML string `json:"html"`
		} `json:"links"`
	} `json:"data"`
}

func (p *Provider) Search(ctx context.Context, q provider.Query, f provider.SubjectFilter) ([]model.Paper, error) {
	p.rateLimiter.Wait()

	osfProv := f.OSFProvider
	if osfProv == "" {
		osfProv = providerID
	}

	params := url.Values{}
	params.Set("filter[provider]", osfProv)
	if q.Keywords != "" {
		params.Set("filter[title]", q.Keywords)
	}
	params.Set("page[size]", fmt.Sprintf("%d", q.Max))

	reqURL := p.baseURL + "/preprints/?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("socarxiv: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("socarxiv: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("socarxiv: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("socarxiv: reading response: %w", err)
	}

	var osfResp osfResponse
	if err := json.Unmarshal(body, &osfResp); err != nil {
		return nil, fmt.Errorf("socarxiv: parsing JSON: %w", err)
	}

	papers := make([]model.Paper, 0, len(osfResp.Data))
	for _, d := range osfResp.Data {
		published := d.Attributes.DatePublished
		if len(published) > 10 {
			published = published[:10]
		}
		pageURL := d.Links.HTML
		if pageURL == "" {
			pageURL = "https://osf.io/preprints/" + osfProv + "/" + d.ID
		}
		papers = append(papers, model.Paper{
			ID:        osfProv + "." + d.ID,
			Title:     d.Attributes.Title,
			Abstract:  d.Attributes.Description,
			Published: published,
			DOI:       d.Attributes.DOI,
			AbsUrl:    pageURL,
			Source:    osfProv,
			SourceURL: pageURL,
		})
	}
	return papers, nil
}

func (p *Provider) DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error) {
	if paper.PDFUrl == "" {
		return nil, fmt.Errorf("%s: no direct PDF URL for %s — visit %s", paper.Source, paper.ID, paper.SourceURL)
	}
	p.rateLimiter.Wait()
	req, err := http.NewRequestWithContext(ctx, "GET", paper.PDFUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", api.UserAgent)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: download HTTP %d", paper.Source, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/provider/socarxiv/... -v -race
```
Expected: 4 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/socarxiv/
git commit -m "feat(provider/socarxiv): implement SocArXiv via OSF API"
```

---

## Task 10: EdArXiv Provider

**Files:**
- Create: `internal/provider/edarxiv/provider.go`
- Create: `internal/provider/edarxiv/provider_test.go`

EdArXiv uses the same OSF API as SocArXiv — only the `provider` ID differs.

- [ ] **Step 1: Write failing test** (same structure as SocArXiv test, `providerID = "edarxiv"`, source field = "edarxiv")

Copy `socarxiv/provider_test.go` to `edarxiv/provider_test.go`, replace:
- package name: `edarxiv_test`
- import path: `edarxivprovider "github.com/joejiang/arxs/internal/provider/edarxiv"`
- `socarxivprovider.New` → `edarxivprovider.New`
- `sampleOSFResponse("socarxiv")` → `sampleOSFResponse("edarxiv")`
- `source != "socarxiv"` → `source != "edarxiv"`
- `OSFProvider: "socarxiv"` → `OSFProvider: "edarxiv"`

- [ ] **Step 2: Implement edarxiv provider**

Copy `socarxiv/provider.go` to `edarxiv/provider.go`, replace:
- `package socarxiv` → `package edarxiv`
- `const providerID = "socarxiv"` → `const providerID = "edarxiv"`
- `provider.ProviderSocArxiv` → `provider.ProviderEdArxiv`
- `"socarxiv: "` error prefixes → `"edarxiv: "`

- [ ] **Step 3: Run tests**

```bash
go test ./internal/provider/edarxiv/... -v -race
```
Expected: 4 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/edarxiv/
git commit -m "feat(provider/edarxiv): implement EdArXiv via OSF API"
```

---

## Task 11: OpenAlex Provider

**Files:**
- Create: `internal/provider/openalex/provider.go`
- Create: `internal/provider/openalex/provider_test.go`

**Note:** OpenAlex returns abstracts as inverted indexes (`abstract_inverted_index`), not plain text. Must reconstruct to string.

- [ ] **Step 1: Write the failing test**

```go
// internal/provider/openalex/provider_test.go
package openalex_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joejiang/arxs/internal/provider"
	oap "github.com/joejiang/arxs/internal/provider/openalex"
)

func sampleOpenAlexResponse() []byte {
	resp := map[string]any{
		"results": []map[string]any{
			{
				"id":    "https://openalex.org/W2741809807",
				"doi":   "https://doi.org/10.1016/j.econ.2025.01.003",
				"title": "Economic Impacts of AI",
				"authorships": []map[string]any{
					{"author": map[string]any{"display_name": "Smith, Jane"}},
				},
				"abstract_inverted_index": map[string]any{
					"We":     []int{0},
					"study":  []int{1},
					"AI":     []int{2},
					"impacts": []int{3},
				},
				"publication_date": "2025-01-20",
				"primary_location": map[string]any{
					"landing_page_url": "https://doi.org/10.1016/j.econ.2025.01.003",
					"pdf_url":          nil,
				},
				"best_oa_location": map[string]any{
					"pdf_url": "https://repo.edu/paper.pdf",
				},
				"cited_by_count": 42,
			},
		},
		"meta": map[string]any{"count": 1},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestOpenAlexProvider_Search_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(sampleOpenAlexResponse())
	}))
	defer srv.Close()

	p := oap.New(oap.WithBaseURL(srv.URL), oap.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "AI", Max: 5}, provider.SubjectFilter{OpenAlexConcepts: []string{"C162324750"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
	if papers[0].Source != "openalex" {
		t.Errorf("expected source=openalex, got %s", papers[0].Source)
	}
	if papers[0].Citations != 42 {
		t.Errorf("expected citations=42, got %d", papers[0].Citations)
	}
	if papers[0].Abstract == "" {
		t.Error("expected reconstructed abstract, got empty")
	}
	if papers[0].PDFUrl != "https://repo.edu/paper.pdf" {
		t.Errorf("unexpected pdf_url: %s", papers[0].PDFUrl)
	}
}

func TestOpenAlexProvider_Search_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", 429)
	}))
	defer srv.Close()

	p := oap.New(oap.WithBaseURL(srv.URL), oap.WithRateInterval(0))
	_, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAlexProvider_Search_Empty(t *testing.T) {
	resp := map[string]any{"results": []any{}, "meta": map[string]any{"count": 0}}
	data, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()

	p := oap.New(oap.WithBaseURL(srv.URL), oap.WithRateInterval(0))
	papers, err := p.Search(context.Background(), provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(papers) != 0 {
		t.Errorf("expected 0, got %d", len(papers))
	}
}

func TestOpenAlexProvider_Search_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	p := oap.New(oap.WithBaseURL(srv.URL), oap.WithRateInterval(0))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := p.Search(ctx, provider.Query{Keywords: "x", Max: 1}, provider.SubjectFilter{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
```

- [ ] **Step 2: Implement openalex provider**

```go
// internal/provider/openalex/provider.go
package openalex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/joejiang/arxs/internal/api"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/provider"
)

const defaultBaseURL = "https://api.openalex.org"

type Option func(*Provider)

func WithBaseURL(u string) Option         { return func(p *Provider) { p.baseURL = u } }
func WithRateInterval(d time.Duration) Option {
	return func(p *Provider) { p.rateLimiter = api.NewRateLimiter(d) }
}

type Provider struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *api.RateLimiter
}

func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: api.NewRateLimiter(100 * time.Millisecond),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) ID() provider.ProviderID { return provider.ProviderOpenAlex }

type openAlexResponse struct {
	Results []struct {
		ID    string `json:"id"`
		DOI   string `json:"doi"`
		Title string `json:"title"`
		Authorships []struct {
			Author struct {
				DisplayName string `json:"display_name"`
			} `json:"author"`
		} `json:"authorships"`
		AbstractInvertedIndex map[string][]int `json:"abstract_inverted_index"`
		PublicationDate       string           `json:"publication_date"`
		PrimaryLocation       *struct {
			LandingPageURL string  `json:"landing_page_url"`
			PDFURL         *string `json:"pdf_url"`
		} `json:"primary_location"`
		BestOALocation *struct {
			PDFURL *string `json:"pdf_url"`
		} `json:"best_oa_location"`
		CitedByCount int `json:"cited_by_count"`
	} `json:"results"`
}

func (p *Provider) Search(ctx context.Context, q provider.Query, f provider.SubjectFilter) ([]model.Paper, error) {
	p.rateLimiter.Wait()

	params := url.Values{}
	// Build filter: concepts + open access
	var filters []string
	for _, c := range f.OpenAlexConcepts {
		filters = append(filters, "concepts.id:"+c)
	}
	filters = append(filters, "is_oa:true")
	params.Set("filter", strings.Join(filters, ","))
	if q.Keywords != "" {
		params.Set("search", q.Keywords)
	}
	params.Set("per_page", fmt.Sprintf("%d", q.Max))
	params.Set("mailto", "arxs")

	reqURL := p.baseURL + "/works?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("openalex: %w", err)
	}
	req.Header.Set("User-Agent", api.UserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openalex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openalex: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openalex: reading response: %w", err)
	}

	var oar openAlexResponse
	if err := json.Unmarshal(body, &oar); err != nil {
		return nil, fmt.Errorf("openalex: parsing JSON: %w", err)
	}

	papers := make([]model.Paper, 0, len(oar.Results))
	for _, r := range oar.Results {
		authors := make([]string, len(r.Authorships))
		for i, a := range r.Authorships {
			authors[i] = a.Author.DisplayName
		}

		abstract := reconstructAbstract(r.AbstractInvertedIndex)

		// Prefer best OA PDF, fall back to primary location PDF
		pdfURL := ""
		if r.BestOALocation != nil && r.BestOALocation.PDFURL != nil {
			pdfURL = *r.BestOALocation.PDFURL
		} else if r.PrimaryLocation != nil && r.PrimaryLocation.PDFURL != nil {
			pdfURL = *r.PrimaryLocation.PDFURL
		}

		landingURL := ""
		if r.PrimaryLocation != nil {
			landingURL = r.PrimaryLocation.LandingPageURL
		}

		// Strip "https://doi.org/" prefix from DOI if present
		doi := strings.TrimPrefix(r.DOI, "https://doi.org/")

		// Use OpenAlex work ID as paper ID
		workID := strings.TrimPrefix(r.ID, "https://openalex.org/")

		papers = append(papers, model.Paper{
			ID:        "openalex." + workID,
			Title:     r.Title,
			Authors:   authors,
			Abstract:  abstract,
			Published: r.PublicationDate,
			DOI:       doi,
			PDFUrl:    pdfURL,
			AbsUrl:    landingURL,
			Citations: r.CitedByCount,
			Source:    "openalex",
			SourceURL: landingURL,
		})
	}
	return papers, nil
}

// reconstructAbstract converts OpenAlex inverted index back to a string.
func reconstructAbstract(index map[string][]int) string {
	if len(index) == 0 {
		return ""
	}
	type wordPos struct {
		word string
		pos  int
	}
	var pairs []wordPos
	for word, positions := range index {
		for _, pos := range positions {
			pairs = append(pairs, wordPos{word, pos})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].pos < pairs[j].pos })
	words := make([]string, len(pairs))
	for i, p := range pairs {
		words[i] = p.word
	}
	return strings.Join(words, " ")
}

func (p *Provider) DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error) {
	if paper.PDFUrl == "" {
		return nil, fmt.Errorf("openalex: no direct PDF for %s — visit %s", paper.ID, paper.SourceURL)
	}
	p.rateLimiter.Wait()
	req, err := http.NewRequestWithContext(ctx, "GET", paper.PDFUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", api.UserAgent)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openalex: download HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/provider/openalex/... -v -race
```
Expected: 4 tests PASS.

- [ ] **Step 4: Run all provider tests together**

```bash
go test ./internal/provider/... -v -race 2>&1 | grep -E "PASS|FAIL|ok"
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/openalex/
git commit -m "feat(provider/openalex): implement OpenAlex provider with abstract reconstruction"
```

---

## Task 12: Orchestrator

**Files:**
- Create: `internal/orchestrator/search.go`
- Create: `internal/orchestrator/search_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/orchestrator/search_test.go
package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/joejiang/arxs/internal/log"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/orchestrator"
	"github.com/joejiang/arxs/internal/provider"
)

// mockProvider is a controllable Provider for testing.
type mockProvider struct {
	id      provider.ProviderID
	papers  []model.Paper
	err     error
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
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./internal/orchestrator/... 2>&1 | head -5
```

- [ ] **Step 3: Implement orchestrator**

```go
// internal/orchestrator/search.go
package orchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/joejiang/arxs/internal/log"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/provider"
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
		"before": countAll(byID), "after": total,
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
```

- [ ] **Step 4: Run tests with -race**

```bash
go test ./internal/orchestrator/... -v -race
```
Expected: 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/
git commit -m "feat(orchestrator): concurrent fan-out with dedup and grouped results"
```

---

## Task 13: CLI — root.go --debug flag

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Add --debug persistent flag and PersistentPreRunE**

In `cmd/root.go`, add:
```go
import (
    "fmt"
    "os"

    "github.com/joejiang/arxs/internal/log"
    "github.com/spf13/cobra"
)

var flagDebug bool

func init() {
    rootCmd.Version = version
    rootCmd.SetVersionTemplate(...)

    rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable structured JSON debug logging to stderr (or set ARXS_DEBUG=1)")

    rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
        enabled := flagDebug || os.Getenv("ARXS_DEBUG") == "1"
        logger := log.New(enabled)
        cmd.SetContext(log.WithLogger(cmd.Context(), logger))
        return nil
    }
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/root.go
git commit -m "feat(cli): add --debug persistent flag with context-based logger injection"
```

---

## Task 14: CLI — search.go refactor

**Files:**
- Modify: `cmd/search.go`

- [ ] **Step 1: Change flagSubjects from string to []string**

In `cmd/search.go`:
```go
// Change declaration
var flagSubjects []string  // was: string

// Change flag registration (in init)
searchCmd.Flags().StringArrayVarP(&flagSubjects, "subject", "s", nil,
    "Subject categories (repeatable, OR): -s cs.AI -s q-fin. Also: -s cs.AI,q-fin")
```

- [ ] **Step 2: Replace runSearch with orchestrator-based implementation**

Replace `runSearch` body:

```go
func runSearch(cmd *cobra.Command, args []string) error {
    if flagKey == "" && flagTitle == "" && flagAbs == "" && flagAuthor == "" {
        return fmt.Errorf("at least one search term is required (-k, -t, -b, or -a)")
    }
    if flagRecent != "" && (flagFrom != "" || flagTo != "") {
        return fmt.Errorf("--recent cannot be used with --from/--to")
    }
    if flagMax < 1 || flagMax > 2000 {
        return fmt.Errorf("--max must be between 1 and 2000")
    }

    from, to := flagFrom, flagTo
    if flagRecent != "" {
        var err error
        from, to, err = parseRecent(flagRecent)
        if err != nil {
            return err
        }
    }

    // Build provider.Query
    terms := make(map[string]string)
    if flagKey != "" { terms["all"] = flagKey }
    if flagTitle != "" { terms["title"] = flagTitle }
    if flagAbs != "" { terms["abs"] = flagAbs }
    if flagAuthor != "" { terms["author"] = flagAuthor }

    // Keywords = joined terms values for non-arXiv sources
    var kwParts []string
    for _, v := range terms {
        kwParts = append(kwParts, v)
    }
    keywords := strings.Join(kwParts, " ")

    q := provider.Query{
        Terms: terms, Op: flagOp, Keywords: keywords,
        From: from, To: to, Max: flagMax,
        SortBy: flagSort, SortOrder: flagSortOrder,
    }

    logger := log.FromContext(cmd.Context())

    // If no subjects: arXiv-only (backward compat)
    if len(flagSubjects) == 0 {
        return runSearchArxivOnly(cmd, q, from, to, logger)
    }

    // Subject-based multi-source search
    lookup, err := subject.Lookup(flagSubjects)
    if err != nil {
        return err
    }

    // Cache key
    cacheKey := buildMultiCacheKey(flagSubjects, q)
    if !flagNoCache {
        cacheDir := filepath.Join(".", ".arxs-cache")
        c := cache.New(cacheDir)
        if cached, ok := c.GetMulti(cacheKey); ok {
            fmt.Fprintf(os.Stderr, "Using cached results from today.\n")
            return outputMultiResults(cached)
        }
    }

    // Build providers in lookup order
    allProviders := buildProviders()
    var providers []provider.Provider
    for _, id := range lookup.Providers {
        if p, ok := allProviders[id]; ok {
            providers = append(providers, p)
        }
    }

    result, err := orchestrator.Search(cmd.Context(), providers, q, lookup.Filter, logger)
    if err != nil {
        return err
    }
    result.Query = model.QueryMeta{
        Terms: terms, Subjects: flagSubjects, Op: flagOp,
        From: from, To: to, Max: flagMax,
        SearchedAt: time.Now().UTC().Format(time.RFC3339),
    }

    // Fetch citation counts for arXiv papers (non-fatal)
    var arxivPapers []model.Paper
    for _, g := range result.Groups {
        if g.Source == "arxiv" {
            arxivPapers = g.Papers
        }
    }
    if len(arxivPapers) > 0 {
        fmt.Fprintf(os.Stderr, "Fetching citation counts...\n")
        cf := api.NewCitationFetcher()
        _ = cf.FetchCitations(arxivPapers)
    }

    if !flagNoCache {
        cacheDir := filepath.Join(".", ".arxs-cache")
        c := cache.New(cacheDir)
        _ = c.SetMulti(cacheKey, result)
    }

    if err := store.WriteMultiSourceResult(flagOutput, result); err != nil {
        return fmt.Errorf("writing results: %w", err)
    }

    return outputMultiResults(result)
}

func buildMultiCacheKey(subjects []string, q provider.Query) string {
    return fmt.Sprintf("%v|%s|%s|%s|%d", subjects, q.Keywords, q.From, q.To, q.Max)
}

// buildProviders constructs a map of all available providers.
func buildProviders() map[provider.ProviderID]provider.Provider {
    return map[provider.ProviderID]provider.Provider{
        provider.ProviderArxiv:    arxivprovider.New(),
        provider.ProviderZenodo:   zenodoprovider.New(),
        provider.ProviderSocArxiv: socarxivprovider.New(),
        provider.ProviderEdArxiv:  edarxivprovider.New(),
        provider.ProviderOpenAlex: openalexprovider.New(),
    }
}

func outputMultiResults(result *model.MultiSourceResult) error {
    totalSources := len(result.Groups)
    fmt.Printf("Found %d papers across %d sources (after dedup), saved to %s\n\n",
        result.Total, totalSources, flagOutput)

    if result.Total == 0 {
        fmt.Println("No results. Try broadening your search terms or removing -s filters.")
        return nil
    }

    globalIdx := 1
    for _, g := range result.Groups {
        fmt.Printf("[%s — %d papers]\n", g.Source, g.Count)
        fmt.Printf(" %-4s %-12s %-10s %-7s %s\n", "#", "Published", "Category", "Cited", "Title")
        for _, p := range g.Papers {
            published := p.Published
            if len(published) >= 10 { published = published[:10] }
            cat := p.Source
            if len(p.Categories) > 0 { cat = p.Categories[0] }
            cited := "-"
            if p.Citations > 0 { cited = fmt.Sprintf("%d", p.Citations) }
            fmt.Printf(" %-4d %-12s %-10s %-7s %s\n", globalIdx, published, cat, cited, p.Title)
            globalIdx++
        }
        fmt.Println()
    }
    return nil
}
```

- [ ] **Step 2b: Add runSearchArxivOnly (backward compat path)**

Add this function to `cmd/search.go` — it is the old single-source arXiv flow, minimally adapted:

```go
// runSearchArxivOnly handles the no-subjects case using the original arXiv-only flow.
func runSearchArxivOnly(cmd *cobra.Command, q provider.Query, from, to string, logger *log.Logger) error {
    params := api.QueryParams{
        Terms: q.Terms, Subjects: nil, Op: q.Op,
        From: from, To: to, Max: q.Max,
        SortBy: q.SortBy, SortOrder: q.SortOrder,
    }

    var c *cache.Cache
    if !flagNoCache {
        cacheDir := filepath.Join(".", ".arxs-cache")
        c = cache.New(cacheDir)
        cacheKey := api.BuildQueryURL(params)
        if cached, ok := c.Get(cacheKey); ok {
            fmt.Fprintf(os.Stderr, "Using cached results from today.\n")
            return outputResults(cached)
        }
    }

    client := api.NewClient()
    result, err := client.Search(params)
    if err != nil {
        return err
    }

    if len(result.Papers) > 0 {
        fmt.Fprintf(os.Stderr, "Fetching citation counts...\n")
        cf := api.NewCitationFetcher()
        _ = cf.FetchCitations(result.Papers)
    }

    result.Query.From = from
    result.Query.To = to

    if flagSort == "citations" {
        sortByCitations(result.Papers)
    }

    if c != nil {
        cacheKey := api.BuildQueryURL(params)
        _ = c.Set(cacheKey, result)
    }

    return outputResults(result)
}
```

- [ ] **Step 3: Build check**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmd/search.go
git commit -m "feat(cli/search): multi-source search with -s StringArray and grouped output"
```

---

## Task 15: CLI — download.go refactor

**Files:**
- Modify: `cmd/download.go`

- [ ] **Step 1: Update runDownload to read MultiSourceResult**

Replace `runDownload` file-reading logic:

```go
func runDownload(cmd *cobra.Command, args []string) error {
    if !flagDownloadAll && len(args) == 0 {
        return fmt.Errorf("specify paper numbers or use --all")
    }

    // Try MultiSourceResult first, fall back to legacy SearchResult
    allPapers, err := loadPapersFromFile(flagDownloadFile)
    if err != nil {
        return fmt.Errorf("cannot read %s: %w\nRun 'arxs search' first.", flagDownloadFile, err)
    }
    // ... rest of existing download logic using allPapers slice
}

func loadPapersFromFile(path string) ([]model.Paper, error) {
    // Try MultiSourceResult
    if multi, err := store.ReadMultiSourceResult(path); err == nil {
        return multi.AllPapers(), nil
    }
    // Fall back to legacy SearchResult
    result, err := store.ReadResults(path)
    if err != nil {
        return nil, err
    }
    return result.Papers, nil
}
```

- [ ] **Step 2: Update downloadPDF to dispatch by source**

```go
func downloadPDF(ctx context.Context, providers map[provider.ProviderID]provider.Provider, paper model.Paper) error {
    filename := sanitizeFilename(paper.ID) + ".pdf"
    path := filepath.Join(flagDownloadDir, filename)

    if !flagDownloadOverwrite {
        if _, err := os.Stat(path); err == nil {
            fmt.Printf("skip: %s (already exists)\n", filename)
            return nil
        }
    }

    fmt.Printf("downloading: %s ...", filename)

    p, ok := providers[provider.ProviderID(paper.Source)]
    if !ok {
        // Fall back to arXiv client for unknown sources
        p = providers[provider.ProviderArxiv]
    }

    data, err := p.DownloadPDF(ctx, paper)
    if err != nil {
        fmt.Println(" FAILED")
        // If no PDF URL, print advisory
        if paper.PDFUrl == "" {
            fmt.Printf("      visit: %s\n", paper.SourceURL)
        }
        return err
    }

    if err := os.WriteFile(path, data, 0644); err != nil {
        fmt.Println(" FAILED")
        return err
    }
    fmt.Println(" done")
    return nil
}
```

- [ ] **Step 3: Update saveAbstract to include source attribution**

```go
func saveAbstract(paper model.Paper) error {
    filename := sanitizeFilename(paper.ID) + ".txt"
    path := filepath.Join(flagDownloadDir, filename)

    if !flagDownloadOverwrite {
        if _, err := os.Stat(path); err == nil {
            fmt.Printf("skip: %s (already exists)\n", filename)
            return nil
        }
    }

    sourceURL := paper.SourceURL
    if sourceURL == "" {
        sourceURL = paper.AbsUrl
    }

    content := fmt.Sprintf(
        "Title: %s\nAuthors: %s\nID: %s\nURL: %s\n\nAbstract:\n%s\n\n---\nSource: %s (%s)\nRetrieved via: arxs v%s | %s\n",
        paper.Title,
        joinStrings(paper.Authors),
        paper.ID,
        paper.AbsUrl,
        paper.Abstract,
        paper.Source,
        sourceURL,
        version,
        time.Now().Format("2006-01-02"),
    )

    if err := os.WriteFile(path, []byte(content), 0644); err != nil {
        return err
    }
    fmt.Printf("saved: %s\n", filename)
    return nil
}
```

- [ ] **Step 4: Build and smoke test**

```bash
go build ./...
go test ./... -race 2>&1 | grep -E "FAIL|ok"
```
Expected: all tests pass, binary builds.

- [ ] **Step 5: Commit**

```bash
git add cmd/download.go
git commit -m "feat(cli/download): dispatch PDF download by source, add source attribution to abstracts"
```

---

## Task 16: Update README + add CLAUDE.md skill

**Files:**
- Modify: `README.md`
- Modify: `README_CN.md`
- Create: `CLAUDE.md`

- [ ] **Step 1: Update README.md subject section**

Replace the old subject table with:

```markdown
## Subject Categories (-s)

`-s` supports arXiv codes and discipline aliases. Multiple `-s` flags use OR logic.

### Quick Aliases

| Alias | Domain | Primary Sources |
|-------|--------|----------------|
| `cs` | Computer Science | arXiv › Zenodo › SocArXiv |
| `physics` | All Physics | arXiv › Zenodo |
| `math` | Mathematics | arXiv › Zenodo |
| `stat` | Statistics | arXiv › Zenodo |
| `q-fin` | Quantitative Finance | arXiv › OpenAlex › Zenodo |
| `econ` | Economics | arXiv › OpenAlex › Zenodo |
| `q-bio` | Quantitative Biology | arXiv › Zenodo |
| `eess` | Electrical Engineering | arXiv › Zenodo |
| `sociology` | Sociology | SocArXiv › OpenAlex › Zenodo |
| `education` | Education | EdArXiv › SocArXiv › Zenodo |
| `philosophy` | Philosophy | OpenAlex › SocArXiv › Zenodo |
| `law` | Law | SocArXiv › OpenAlex › Zenodo |
| `psychology` | Psychology | SocArXiv › EdArXiv › Zenodo |

### arXiv Codes
Also accepts any arXiv category code: `cs.AI`, `cs.LG`, `cs.CL`, `hep-th`, `quant-ph`, `math.AG`, etc.

### Examples

```bash
arxs search -k "transformer" -s cs.AI
arxs search -k "inequality" -s sociology -s law
arxs search -k "option pricing" -s q-fin --recent 12m
arxs search -k "curriculum" -s education -s psychology
```
```

- [ ] **Step 2: Create CLAUDE.md** (project skill for Claude Code)

```markdown
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
3. Register in `subject/registry.go` entries
4. Add to `buildProviders()` map in `cmd/search.go`

## Debug Logging
```bash
arxs search -k "test" -s cs.AI --debug 2>debug.log
# or
ARXS_DEBUG=1 arxs search -k "test" -s cs.AI 2>debug.log
```
```

- [ ] **Step 3: Commit**

```bash
git add README.md README_CN.md CLAUDE.md
git commit -m "docs: update README with multi-source subjects, add CLAUDE.md project guide"
```

---

## Final Verification

- [ ] **Run full test suite with race detector**

```bash
go test ./... -race -count=1
```
Expected: all PASS, no race conditions.

- [ ] **Build for current platform**

```bash
go build -o arxs-test . && ./arxs-test --help
```
Expected: help text shows updated `-s` description.

- [ ] **Smoke test subject lookup**

```bash
./arxs-test search -k "transformer" -s cs.AI --debug 2>&1 | head -20
```
Expected: structured JSON log lines with `trace_id`, `provider_start` events for arxiv + zenodo + socarxiv.

- [ ] **Final commit**

```bash
git tag v2.0.0
```
