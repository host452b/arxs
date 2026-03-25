package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List papers from search results",
	Long: `List papers from a search results JSON file.

Supports both arXiv-only results and multi-source results (produced by search -s).
Papers are numbered starting from 1 — use these numbers with 'arxs download'.

OUTPUT COLUMNS:
  #         Result number (use with 'arxs download')
  Published Date (YYYY-MM-DD)
  Category  arXiv category (e.g. cs.AI) or source name for non-arXiv papers
  Cited     Citation count (arXiv papers only; - if unavailable)
  Title     Full paper title

DEFAULTS:
  -f arxiv-results.json   Results file
  -n 0                    Show all results (0 = no limit)

Examples:
  arxs list                        # List all results
  arxs list --verbose              # Show with abstracts, authors, URLs
  arxs list -n 10                  # First 10 papers only
  arxs list -f ./papers.json       # From specific file`,
	RunE: runList,
}

var (
	flagListFile    string
	flagListVerbose bool
	flagListLimit   int
)

func init() {
	listCmd.Flags().StringVarP(&flagListFile, "file", "f", "arxiv-results.json", "JSON results file")
	listCmd.Flags().BoolVarP(&flagListVerbose, "verbose", "v", false, "Show abstracts")
	listCmd.Flags().IntVarP(&flagListLimit, "limit", "n", 0, "Show first N results (0 = all)")

	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	papers, err := loadPapersFromFile(flagListFile)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w\nRun 'arxs search' first to create results.", flagListFile, err)
	}

	if len(papers) == 0 {
		fmt.Println("No papers in results file.")
		return nil
	}

	total := len(papers)
	if flagListLimit > 0 && flagListLimit < len(papers) {
		papers = papers[:flagListLimit]
	}

	fmt.Printf("Results from %s (%d papers)\n\n", flagListFile, total)
	fmt.Printf(" %-4s %-12s %-10s %-7s %s\n", "#", "Published", "Category", "Cited", "Title")

	for i, p := range papers {
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
		fmt.Printf(" %-4d %-12s %-10s %-7s %s\n", i+1, published, cat, cited, p.Title)

		if flagListVerbose {
			fmt.Printf("      Authors: %s\n", joinStrings(p.Authors))
			fmt.Printf("      PDF: %s\n", p.PDFUrl)
			fmt.Printf("      Abstract: %s\n\n", truncate(p.Abstract, 200))
		}
	}

	return nil
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
