package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Show tool information and credits",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(`arxs - arXiv paper search CLI tool v%s

Thank you to arXiv for use of its open access interoperability.

API:          https://export.arxiv.org/api/query
Terms of Use: https://arxiv.org/help/api/tou
Rate limit:   3s between requests (enforced)
Source:       https://github.com/joejiang/arxs
`, version)
	},
}

func init() {
	rootCmd.AddCommand(aboutCmd)
}
