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
	"github.com/joejiang/arxs/internal/model"
	"github.com/joejiang/arxs/internal/store"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search arXiv papers",
	Long: `Search arXiv papers by title, abstract, author, or all fields.

SEARCH FLAGS:
  -k   Search all fields (title OR abstract OR author)
  -t   Search by title only
  -b   Search by abstract only
  -a   Search by author only

KEYWORD SYNTAX:
  Words without operators use implicit AND: "reinforcement learning" = "reinforcement and learning"
  Use "or" / "and" to combine: "transformer or attention", "vaswani and hinton"
  Precedence: and > or, so "A or B and C" = "A or (B and C)"

CROSS-FIELD:
  Multiple -k/-t/-b/-a flags default to AND. Use --op or to switch.

DEFAULTS:
  --op and           Cross-field operator
  --max 50           Max results (range: 1-2000)
  --sort submitted   Sort by: relevance, submitted, updated
  --order desc       Sort direction: asc, desc
  -o arxiv-results.json   Output file

Examples:
  arxs search -k "transformer"                               # All fields
  arxs search -t "transformer or attention"                   # Title with OR
  arxs search -t "diffusion model" -a "ho and song"           # Title AND author
  arxs search -t "RLHF" -b "reward model" --op or            # Cross-field OR
  arxs search -k "LLM" -s cs,stat --from 2024-01             # Subject + date
  arxs search -k "quantum computing" --recent 12m            # Recent papers
  arxs search -k "GAN" --max 100 --sort relevance            # More results, by relevance
  arxs search -k "black hole" -s physics -o physics.json     # Custom output file`,
	RunE: runSearch,
}

var (
	flagKey       string
	flagTitle     string
	flagAbs       string
	flagAuthor    string
	flagSubjects  string
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
	searchCmd.Flags().StringVarP(&flagSubjects, "subject", "s", "", "Subject categories (comma-separated: cs,math,physics,...)")
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
	// Validate: at least one search term
	if flagKey == "" && flagTitle == "" && flagAbs == "" && flagAuthor == "" {
		return fmt.Errorf("at least one search term is required (-k, -t, -b, or -a)")
	}

	// Validate: --recent and --from/--to are mutually exclusive
	if flagRecent != "" && (flagFrom != "" || flagTo != "") {
		return fmt.Errorf("--recent cannot be used with --from/--to")
	}

	// Validate: max range
	if flagMax < 1 || flagMax > 2000 {
		return fmt.Errorf("--max must be between 1 and 2000")
	}

	// Parse --recent into from/to
	from, to := flagFrom, flagTo
	if flagRecent != "" {
		var err error
		from, to, err = parseRecent(flagRecent)
		if err != nil {
			return err
		}
	}

	// Build terms map
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

	// Parse subjects
	var subjects []string
	if flagSubjects != "" {
		subjects = strings.Split(flagSubjects, ",")
	}

	params := api.QueryParams{
		Terms:     terms,
		Subjects:  subjects,
		Op:        flagOp,
		From:      from,
		To:        to,
		Max:       flagMax,
		SortBy:    flagSort,
		SortOrder: flagSortOrder,
	}

	// Check cache
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

	// Search
	client := api.NewClient()
	result, err := client.Search(params)
	if err != nil {
		return err
	}

	// Fetch citation counts from Semantic Scholar
	if len(result.Papers) > 0 {
		fmt.Fprintf(os.Stderr, "Fetching citation counts...\n")
		cf := api.NewCitationFetcher()
		_ = cf.FetchCitations(result.Papers) // Non-fatal if it fails
	}

	// Update query metadata
	result.Query.From = from
	result.Query.To = to

	// Sort by citations if requested
	if flagSort == "citations" {
		sortByCitations(result.Papers)
	}

	// Cache result
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
