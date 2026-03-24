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
	Long:  "arxs - Search and download arXiv papers from the command line.",
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
