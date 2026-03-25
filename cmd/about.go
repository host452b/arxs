package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Show tool information and credits",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(`arxs v%s — Academic paper search across 5 sources.

Sources and APIs:
  arXiv       export.arxiv.org/api     rate limit >=3s, no scraping
  Zenodo      zenodo.org/api/records   open access only
  SocArXiv    api.osf.io/v2/preprints  provider=socarxiv
  EdArXiv     api.osf.io/v2/preprints  provider=edarxiv
  OpenAlex    api.openalex.org/works   open access filter

Compliance:
  Same-day query caching (avoids redundant requests)
  Custom User-Agent on all requests
  arXiv API Terms of Use: https://arxiv.org/help/api/tou

Source: https://github.com/host452b/arxs
Thank you to arXiv for use of its open access interoperability.
`, version)
	},
}

func init() {
	rootCmd.AddCommand(aboutCmd)
}
