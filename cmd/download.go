package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/host452b/arxs/internal/model"
	"github.com/host452b/arxs/internal/provider"
	"github.com/host452b/arxs/internal/store"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download [paper numbers...]",
	Short: "Download papers (PDF or abstract)",
	Long: `Download PDFs or abstracts from search results.

Files are named {arXiv_ID}.pdf or {arXiv_ID}.txt.
Existing files are skipped unless --overwrite is set.
Downloads >10 papers with --all will prompt for confirmation.

DEFAULTS:
  -f arxiv-results.json   Results file
  -d .                    Save to current directory

Examples:
  arxs download 1 3 5              # Download PDFs for paper #1, #3, #5
  arxs download --all               # Download all PDFs
  arxs download --abs-only 2 4      # Save abstracts as .txt
  arxs download 1 --dir ./papers    # Save to specific directory
  arxs download 1 -f ./gan.json     # From specific results file
  arxs download 1 --overwrite       # Overwrite existing files`,
	RunE: runDownload,
}

var (
	flagDownloadFile      string
	flagDownloadDir       string
	flagDownloadAbsOnly   bool
	flagDownloadAll       bool
	flagDownloadOverwrite bool
)

func init() {
	downloadCmd.Flags().StringVarP(&flagDownloadFile, "file", "f", "arxiv-results.json", "JSON results file")
	downloadCmd.Flags().StringVarP(&flagDownloadDir, "dir", "d", ".", "Save directory")
	downloadCmd.Flags().BoolVar(&flagDownloadAbsOnly, "abs-only", false, "Download abstracts only (.txt)")
	downloadCmd.Flags().BoolVar(&flagDownloadAll, "all", false, "Download all results")
	downloadCmd.Flags().BoolVar(&flagDownloadOverwrite, "overwrite", false, "Overwrite existing files")

	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	if !flagDownloadAll && len(args) == 0 {
		return fmt.Errorf("specify paper numbers or use --all")
	}

	allPapers, err := loadPapersFromFile(flagDownloadFile)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w\nRun 'arxs search' first.", flagDownloadFile, err)
	}

	if len(allPapers) == 0 {
		return fmt.Errorf("no papers in results file")
	}

	// Determine which papers to download
	var indices []int
	if flagDownloadAll {
		for i := range allPapers {
			indices = append(indices, i)
		}
	} else {
		for _, arg := range args {
			n, err := strconv.Atoi(arg)
			if err != nil || n < 1 || n > len(allPapers) {
				return fmt.Errorf("invalid paper number: %s (valid range: 1-%d)", arg, len(allPapers))
			}
			indices = append(indices, n-1) // Convert to 0-based
		}
	}

	// Confirm large downloads
	if flagDownloadAll && len(indices) > 10 && !flagDownloadAbsOnly {
		estimatedMin := len(indices) * 3 / 60
		fmt.Printf("About to download %d PDFs (~%d min at 3s interval). Continue? [y/N] ", len(indices), estimatedMin)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Ensure dir exists
	if err := os.MkdirAll(flagDownloadDir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	provs := buildProviders()
	var failed []string
	downloaded := 0

	for _, idx := range indices {
		paper := allPapers[idx]
		if flagDownloadAbsOnly {
			err := saveAbstract(paper)
			if err != nil {
				failed = append(failed, fmt.Sprintf("#%d %s: %v", idx+1, paper.ID, err))
				continue
			}
		} else {
			err := downloadPDF(cmd.Context(), provs, paper)
			if err != nil {
				failed = append(failed, fmt.Sprintf("#%d %s: %v", idx+1, paper.ID, err))
				continue
			}
		}
		downloaded++
	}

	fmt.Printf("\nDownloaded %d/%d files.\n", downloaded, len(indices))
	if len(failed) > 0 {
		fmt.Println("Failed:")
		for _, f := range failed {
			fmt.Printf("  %s\n", f)
		}
	}

	return nil
}

func loadPapersFromFile(path string) ([]model.Paper, error) {
	// Try MultiSourceResult first (has "groups" field)
	if multi, err := store.ReadMultiSourceResult(path); err == nil && len(multi.Groups) > 0 {
		return multi.AllPapers(), nil
	}
	// Fall back to legacy SearchResult
	result, err := store.ReadResults(path)
	if err != nil {
		return nil, err
	}
	return result.Papers, nil
}

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

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeFilename(s string) string {
	return unsafeChars.ReplaceAllString(s, "_")
}
