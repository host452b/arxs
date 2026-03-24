package cmd

import (
	"fmt"

	"github.com/joejiang/arxs/internal/store"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List search results",
	Long: `List papers from the search results JSON file.

Examples:
  arxs list
  arxs list --verbose
  arxs list -f ./my-results.json -n 10`,
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
	result, err := store.ReadResults(flagListFile)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w\nRun 'arxs search' first to create results.", flagListFile, err)
	}

	papers := result.Papers
	if flagListLimit > 0 && flagListLimit < len(papers) {
		papers = papers[:flagListLimit]
	}

	if len(papers) == 0 {
		fmt.Println("No papers in results file.")
		return nil
	}

	fmt.Printf("Results from %s (%d papers)\n\n", flagListFile, len(result.Papers))
	fmt.Printf(" %-4s %-12s %-10s %s\n", "#", "Published", "Category", "Title")

	for i, p := range papers {
		published := p.Published
		if len(published) >= 10 {
			published = published[:10]
		}
		cat := ""
		if len(p.Categories) > 0 {
			cat = p.Categories[0]
		}
		title := p.Title
		if !flagListVerbose && len(title) > 60 {
			title = title[:57] + "..."
		}
		fmt.Printf(" %-4d %-12s %-10s %s\n", i+1, published, cat, title)

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
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
