package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "1.0.1"

var rootCmd = &cobra.Command{
	Use:   "arxs",
	Short: "arXiv paper search CLI tool",
	Long: `arxs - A CLI tool to search and download arXiv papers.

Uses the official arXiv API (export.arxiv.org) with built-in rate limiting (3s)
and same-day query caching. Respects arXiv robots.txt and API Terms of Use.

COMMANDS:
  search    Search papers by title, abstract, author, or all fields
  list      View saved search results
  download  Download PDFs or abstracts from search results
  about     Show tool info, credits, and compliance details

SEARCH FIELDS:
  -k   Search all fields (title OR abstract OR author)
  -t   Search by title only
  -b   Search by abstract only
  -a   Search by author only

KEYWORD SYNTAX (inspired by pytest -k):
  "transformer"                    Single keyword
  "transformer or attention"       OR: match either
  "vaswani and hinton"             AND: match both
  "A or B and C"                   AND binds tighter: A or (B and C)
  "reinforcement learning"         Implicit AND between words

CROSS-FIELD OPERATORS:
  Multiple -k/-t/-b/-a flags default to AND. Use --op or to switch:
    arxs search -t "RLHF" -b "reward model" --op or

SUBJECT CATEGORIES (-s):
  cs        Computer Science       math    Mathematics
  physics   Physics                stat    Statistics
  eess      EE & Systems Science   econ    Economics
  q-bio     Quantitative Biology   q-fin   Quantitative Finance

  Comma-separated for multiple: -s cs,stat
  Omit -s to search all categories.

DATE FILTERS:
  --from 2024-01 --to 2025-03    Date range (YYYY[-MM[-DD]])
  --recent 12m                    Last 12 months (also: 6m, 1y, 2y)
  (--recent and --from/--to are mutually exclusive)

WORKFLOW:
  1. arxs search -k "your query"      Search and save results to JSON
  2. arxs list                         Browse results in terminal
  3. arxs list --verbose               View with abstracts
  4. arxs download 1 3 5               Download selected PDFs
  5. arxs download --abs-only 2        Save abstract as .txt

OUTPUT:
  Search results are saved to ./arxiv-results.json (override with -o).
  PDFs saved as {arXiv_ID}.pdf, abstracts as {arXiv_ID}.txt.
  All files are stored in the current working directory by default.

COMPLIANCE:
  - Uses official arXiv API only, no web scraping
  - Rate limit: >=3s between requests (hardcoded, non-configurable)
  - Same-day query caching to avoid redundant requests
  - Custom User-Agent header on all requests
  - Complies with arXiv API Terms of Use (https://arxiv.org/help/api/tou)

Thank you to arXiv for use of its open access interoperability.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf(
		"arxs v%s\nThank you to arXiv for use of its open access interoperability.\n", version))
}
