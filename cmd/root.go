package cmd

import (
	"fmt"
	"os"

	"github.com/host452b/arxs/internal/log"
	"github.com/spf13/cobra"
)

const version = "2.0.1"

var flagDebug bool

var rootCmd = &cobra.Command{
	Use:   "arxs",
	Short: "Academic paper search CLI — arXiv, Zenodo, SocArXiv, EdArXiv, OpenAlex",
	Long: `arxs — Search academic papers across 5 sources:
  arXiv, Zenodo, SocArXiv (OSF), EdArXiv (OSF), OpenAlex.

Without -s, searches arXiv only. With -s, routes to relevant sources automatically.

COMMANDS:
  search    Search papers by keyword, title, abstract, or author
  list      View saved search results
  download  Download PDFs or abstracts from search results
  about     Show tool info, credits, and compliance details

SEARCH FIELDS:
  -k   Search all fields (title OR abstract OR author)
  -t   Search by title only
  -b   Search by abstract only
  -a   Search by author only

KEYWORD SYNTAX:
  "transformer"                    Single keyword (implicit AND for multiple words)
  "transformer or attention"       OR: match either
  "vaswani and hinton"             AND: match both
  "A or B and C"                   AND binds tighter → A or (B and C)

CROSS-FIELD:
  Multiple -k/-t/-b/-a flags combine with AND by default.
  Use --op or to switch to OR:  arxs search -t "RLHF" -b "reward model" --op or

SUBJECT CATEGORIES (-s):
  Alias        Domain               Sources
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

  Also accepts arXiv codes: cs.AI, cs.LG, cs.CL, hep-th, quant-ph, math.AG, ...
  Multiple -s flags use OR logic: -s cs.AI -s cs.LG
  Comma-separated also works:    -s cs.AI,cs.LG
  Omit -s entirely to search arXiv only (no subject routing).

DATE FILTERS:
  --from 2024-01 --to 2025-03    Date range (YYYY[-MM[-DD]])
  --recent 12m                    Last 12 months (also: 6m, 1y, 2y)
  (--recent and --from/--to are mutually exclusive)

OUTPUT:
  Results saved to ./arxiv-results.json (override with -o).
  JSON schema: { total, groups: [{ source, count, papers: [{ id, title,
    authors, abstract, published, categories, doi, pdf_url, source_url,
    citations, source }] }] } for multi-source; flat { papers } for arXiv-only.
  PDFs saved as {ID}.pdf, abstracts as {ID}.txt.

WORKFLOW:
  1. arxs search -k "your query"             arXiv-only search
  2. arxs search -k "query" -s cs.AI         Multi-source by subject
  3. arxs list                               Browse results as numbered flat list
  4. arxs list --verbose                     With full abstracts
  5. arxs download 1 3 5                     Download PDFs by result number
  6. arxs download --abs-only 2              Save abstract as .txt

DEBUG:
  --debug flag or ARXS_DEBUG=1 env var → structured JSON logs to stderr.
  arxs search -k "test" -s cs.AI --debug 2>debug.log

COMPLIANCE:
  arXiv:          export.arxiv.org/api — rate limit >=3s, no scraping
  Zenodo:         zenodo.org/api/records — open access only
  SocArXiv/EdArXiv: api.osf.io/v2/preprints — preprints only
  OpenAlex:       api.openalex.org/works — open access only
  Same-day query caching avoids redundant requests.

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
	rootCmd.SilenceUsage = true  // don't dump usage on runtime errors
	rootCmd.SilenceErrors = true // let Execute() print the error once, cleanly
	rootCmd.SetVersionTemplate(fmt.Sprintf(
		"arxs v%s — arXiv, Zenodo, SocArXiv, EdArXiv, OpenAlex\nThank you to arXiv for use of its open access interoperability.\n", version))

	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable structured JSON debug logging to stderr (or set ARXS_DEBUG=1)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		enabled := flagDebug || os.Getenv("ARXS_DEBUG") == "1"
		logger := log.New(enabled)
		cmd.SetContext(log.WithLogger(cmd.Context(), logger))
		return nil
	}
}
