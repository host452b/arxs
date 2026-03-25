package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joejiang/arxs/internal/api"
	"github.com/joejiang/arxs/internal/cache"
	"github.com/joejiang/arxs/internal/log"
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/orchestrator"
	"github.com/joejiang/arxs/internal/provider"
	arxivprovider "github.com/joejiang/arxs/internal/provider/arxiv"
	edarxivprovider "github.com/joejiang/arxs/internal/provider/edarxiv"
	openalexprovider "github.com/joejiang/arxs/internal/provider/openalex"
	socarxivprovider "github.com/joejiang/arxs/internal/provider/socarxiv"
	zenodoprovider "github.com/joejiang/arxs/internal/provider/zenodo"
	"github.com/joejiang/arxs/internal/store"
	"github.com/joejiang/arxs/internal/subject"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search papers across arXiv, Zenodo, SocArXiv, EdArXiv, OpenAlex",
	Long: `Search academic papers by keyword, title, abstract, or author.

Without -s: searches arXiv only (original behavior, backward compatible).
With -s:    routes to relevant sources based on subject, aggregates and deduplicates results.

SEARCH FLAGS:
  -k, --key        Search all fields (title OR abstract OR author)
  -t, --key-title  Search by title only
  -b, --key-abs    Search by abstract only
  -a, --key-author Search by author only

At least one of -k/-t/-b/-a is required.

KEYWORD SYNTAX:
  "transformer"                 Single keyword
  "reinforcement learning"      Implicit AND: finds both words
  "transformer or attention"    OR: match either term
  "vaswani and hinton"          AND: match both terms
  "A or B and C"                Precedence: AND > OR → A or (B and C)

CROSS-FIELD OPERATOR (--op):
  Default AND: all field flags must match.
  --op or: any field flag may match.
  Example: arxs search -t "RLHF" -b "reward model" --op or

SUBJECT CATEGORIES (-s):
  Alias        Domain               Primary Sources
  ──────────────────────────────────────────────────────────────
  cs           Computer Science     arXiv › Zenodo › SocArXiv
  physics      All Physics          arXiv › Zenodo
  math         Mathematics          arXiv › Zenodo
  stat         Statistics           arXiv › Zenodo
  q-fin        Quant Finance        arXiv › OpenAlex › Zenodo
  econ         Economics            arXiv › OpenAlex › Zenodo
  q-bio        Quant Biology        arXiv › Zenodo
  eess         EE & Sys Science     arXiv › Zenodo
  sociology    Sociology            SocArXiv › OpenAlex › Zenodo
  education    Education            EdArXiv › SocArXiv › Zenodo
  philosophy   Philosophy           OpenAlex › SocArXiv › Zenodo
  law          Law                  SocArXiv › OpenAlex › Zenodo
  psychology   Psychology           SocArXiv › EdArXiv › Zenodo

  Also accepts any arXiv category code: cs.AI, cs.LG, cs.CL, hep-th, quant-ph, math.AG, ...
  Multiple -s flags: OR logic  →  -s cs.AI -s cs.LG
  Comma-separated:             →  -s cs.AI,cs.LG
  Omit -s to search arXiv only (no subject routing).

DATE FILTERS:
  --from 2024-01          Start date (YYYY, YYYY-MM, or YYYY-MM-DD)
  --to   2025-03          End date   (same format)
  --recent 12m            Last N months: 6m, 12m, 24m
  --recent 1y             Last N years:  1y, 2y
  (--recent and --from/--to are mutually exclusive)

SORT & LIMITS:
  --max 100               Max results per source (1–2000, default 50)
  --sort submitted        Sort by: submitted, updated, relevance, citations (default: submitted)
  --order desc            Sort order: asc, desc (default: desc)

OUTPUT:
  -o, --output FILE       Output JSON file (default: arxiv-results.json)
  --no-cache              Bypass same-day cache and fetch fresh results

  Multi-source JSON schema (when -s is used):
    { "total": N, "query": {...}, "groups": [
        { "source": "arxiv",   "count": N, "papers": [...] },
        { "source": "zenodo",  "count": N, "papers": [...] },
        { "source": "socarxiv","count": N, "papers": [...] }
    ]}
  arXiv-only JSON schema:
    { "total_results": N, "return_count": N, "query": {...}, "papers": [...] }

  Paper object fields: id, title, authors[], abstract, published, updated,
    categories[], doi, pdf_url, abs_url, source_url, citations, source.

EXAMPLES:
  arxs search -k "transformer"                          # arXiv, all fields
  arxs search -t "transformer or attention"             # title with OR
  arxs search -t "diffusion model" -a "ho and song"     # title AND author
  arxs search -t "RLHF" -b "reward model" --op or      # cross-field OR
  arxs search -k "LLM" -s cs.AI --from 2024-01         # subject + date filter
  arxs search -k "LLM" -s cs.AI -s cs.LG               # multiple subjects (OR)
  arxs search -k "inequality" -s sociology -s law       # social sciences multi-source
  arxs search -k "curriculum" -s education -s psychology # cross-discipline
  arxs search -k "option pricing" -s q-fin --recent 12m # recent finance papers
  arxs search -k "GAN" --max 100 --sort relevance       # relevance sort
  arxs search -k "GAN" --sort citations                 # sort by citation count
  arxs search -k "black hole" -s physics -o papers.json # custom output file`,
	RunE: runSearch,
}

var (
	flagKey       string
	flagTitle     string
	flagAbs       string
	flagAuthor    string
	flagSubjects  []string
	flagOp        string
	flagFrom      string
	flagTo        string
	flagRecent    string
	flagMax       int
	flagOutput    string
	flagNoCache   bool
	flagSort      string
	flagSortOrder string
)

func init() {
	searchCmd.Flags().StringVarP(&flagKey, "key", "k", "", "Search all fields (title OR abstract OR author)")
	searchCmd.Flags().StringVarP(&flagTitle, "key-title", "t", "", "Search by title")
	searchCmd.Flags().StringVarP(&flagAbs, "key-abs", "b", "", "Search by abstract")
	searchCmd.Flags().StringVarP(&flagAuthor, "key-author", "a", "", "Search by author")
	searchCmd.Flags().StringArrayVarP(&flagSubjects, "subject", "s", nil,
		"Subject categories (repeatable, OR): -s cs.AI -s q-fin. Also: -s cs.AI,q-fin")
	searchCmd.Flags().StringVar(&flagOp, "op", "and", "Operator between -k-* fields: and, or")
	searchCmd.Flags().StringVar(&flagFrom, "from", "", "Start date (YYYY[-MM[-DD]])")
	searchCmd.Flags().StringVar(&flagTo, "to", "", "End date (YYYY[-MM[-DD]])")
	searchCmd.Flags().StringVar(&flagRecent, "recent", "", "Recent papers shortcut (e.g. 12m, 6m, 1y)")
	searchCmd.Flags().IntVarP(&flagMax, "max", "m", 50, "Max results (1-2000)")
	searchCmd.Flags().StringVarP(&flagOutput, "output", "o", "arxiv-results.json", "Output JSON file path")
	searchCmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Skip cache")
	searchCmd.Flags().StringVar(&flagSort, "sort", "submitted", "Sort by: relevance, submitted, updated, citations")
	searchCmd.Flags().StringVar(&flagSortOrder, "order", "desc", "Sort order: asc, desc")

	rootCmd.AddCommand(searchCmd)
}

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

	terms := make(map[string]string)
	if flagKey != "" {
		terms["all"] = flagKey
	}
	if flagTitle != "" {
		terms["title"] = flagTitle
	}
	if flagAbs != "" {
		terms["abs"] = flagAbs
	}
	if flagAuthor != "" {
		terms["author"] = flagAuthor
	}

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
			if len(published) >= 10 {
				published = published[:10]
			}
			cat := p.Source
			if len(p.Categories) > 0 {
				cat = p.Categories[0]
			}
			cited := "-"
			if p.Citations > 0 {
				cited = fmt.Sprintf("%d", p.Citations)
			}
			fmt.Printf(" %-4d %-12s %-10s %-7s %s\n", globalIdx, published, cat, cited, p.Title)
			globalIdx++
		}
		fmt.Println()
	}
	return nil
}

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

func outputResults(result *model.SearchResult) error {
	// Write JSON
	if err := store.WriteResults(flagOutput, result); err != nil {
		return fmt.Errorf("writing results: %w", err)
	}

	// Print summary
	if result.TotalResults > result.ReturnCount {
		fmt.Printf("Found %d papers (%d total matches), saved to %s\n\n",
			result.ReturnCount, result.TotalResults, flagOutput)
	} else {
		fmt.Printf("Found %d papers, saved to %s\n\n", result.ReturnCount, flagOutput)
	}

	if len(result.Papers) == 0 {
		fmt.Println("No results. Try broadening your search terms or removing filters.")
		return nil
	}

	// Print table
	fmt.Printf(" %-4s %-12s %-10s %-7s %s\n", "#", "Published", "Category", "Cited", "Title")
	for i, p := range result.Papers {
		published := p.Published
		if len(published) >= 10 {
			published = published[:10]
		}
		cat := ""
		if len(p.Categories) > 0 {
			cat = p.Categories[0]
		}
		cited := "-"
		if p.Citations > 0 {
			cited = fmt.Sprintf("%d", p.Citations)
		}
		fmt.Printf(" %-4d %-12s %-10s %-7s %s\n", i+1, published, cat, cited, p.Title)
	}

	return nil
}

func sortByCitations(papers []model.Paper) {
	sort.Slice(papers, func(i, j int) bool {
		return papers[i].Citations > papers[j].Citations
	})
}

func parseRecent(s string) (string, string, error) {
	now := time.Now()
	to := now.Format("2006-01-02")

	if strings.HasSuffix(s, "m") {
		months := 0
		_, err := fmt.Sscanf(s, "%dm", &months)
		if err != nil || months <= 0 {
			return "", "", fmt.Errorf("invalid --recent format: %q (use e.g. 12m, 6m)", s)
		}
		from := now.AddDate(0, -months, 0).Format("2006-01-02")
		return from, to, nil
	}
	if strings.HasSuffix(s, "y") {
		years := 0
		_, err := fmt.Sscanf(s, "%dy", &years)
		if err != nil || years <= 0 {
			return "", "", fmt.Errorf("invalid --recent format: %q (use e.g. 1y, 2y)", s)
		}
		from := now.AddDate(-years, 0, 0).Format("2006-01-02")
		return from, to, nil
	}

	return "", "", fmt.Errorf("invalid --recent format: %q (use e.g. 12m, 6m, 1y)", s)
}
