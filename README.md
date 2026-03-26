# arxs

**Multi-source academic paper search CLI** — search, browse, and download papers from arXiv, Zenodo, SocArXiv, EdArXiv, and OpenAlex in one command.

> arXiv paper search · literature review tool · preprint downloader · open-access academic search · research paper CLI · citation lookup · arXiv API client

[中文文档](README_CN.md)

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go) ![License](https://img.shields.io/badge/license-MIT-green) ![Sources](https://img.shields.io/badge/sources-arXiv%20%7C%20Zenodo%20%7C%20SocArXiv%20%7C%20EdArXiv%20%7C%20OpenAlex-blue)

## Features

- **5 sources**: arXiv, Zenodo, SocArXiv (OSF), EdArXiv (OSF), OpenAlex — routed automatically by subject
- **Smart dedup**: cross-source deduplication by DOI and title
- **Citation counts**: fetched from Semantic Scholar for arXiv papers
- **Offline-friendly**: same-day query cache, no redundant API calls
- **AI-agent ready**: `--list-subjects`, `--debug`, JSON output schema documented in `--help`

> Thank you to arXiv for use of its open access interoperability.

## Install

### Option 1 — Go install (recommended if you have Go ≥ 1.21)

```bash
go install github.com/host452b/arxs/v2@latest
```

`arxs` is installed to `$(go env GOPATH)/bin`. Make sure that directory is in your `$PATH`:

```bash
# Add to ~/.bashrc or ~/.zshrc if not already present:
export PATH="$PATH:$(go env GOPATH)/bin"
```

Verify:

```bash
arxs --version   # arxs v2.0.2
```

### Option 2 — Shell script (macOS / Linux, no Go required)

Downloads the pre-built binary for your platform:

```bash
curl -fsSL https://raw.githubusercontent.com/host452b/arxs/main/install.sh | sh
```

The script installs to `/usr/local/bin/arxs` (requires sudo on some systems). To install to a custom directory:

```bash
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/host452b/arxs/main/install.sh | sh
```

### Option 3 — Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/host452b/arxs/main/install.ps1 | iex
```

Installs to `%USERPROFILE%\AppData\Local\Microsoft\WindowsApps`. Open a new terminal after installation.

### Option 4 — Build from source

```bash
# Requires Go 1.21+
git clone https://github.com/host452b/arxs.git
cd arxs
go build -o arxs .          # Linux/macOS
go build -o arxs.exe .      # Windows
```

Move the binary to a directory in your `$PATH`:

```bash
mv arxs /usr/local/bin/     # or: ~/.local/bin/, ~/bin/, etc.
```

### Option 5 — Pin a specific version

```bash
go install github.com/host452b/arxs/v2@v2.0.2
```

### Verify installation

```bash
arxs --version        # shows version
arxs --help           # shows full usage
arxs search --help    # search-specific flags and examples
arxs search --list-subjects   # list all 53 valid subject codes
```

## Quick Start

```bash
# Search for papers
arxs search -k "transformer"

# View results
arxs list

# Download papers
arxs download 1 3 5
```

## Usage

### search — Search papers

#### Basic search

```bash
# Search all fields (title, abstract, author)
arxs search -k "transformer"

# Search by title only
arxs search -t "transformer"

# Search by abstract only
arxs search -b "reinforcement learning"

# Search by author only
arxs search -a "vaswani"
```

#### Combine keywords with or / and

Within a single field, use `or` and `and` to combine keywords (inspired by pytest `-k`):

```bash
# Title contains "transformer" OR "attention"
arxs search -t "transformer or attention"

# Author contains both "vaswani" AND "hinton"
arxs search -a "vaswani and hinton"

# Multiple OR
arxs search -t "transformer or attention or self-attention"
```

**Operator precedence**: `and` binds tighter than `or`:
- `"A or B and C"` is equivalent to `"A or (B and C)"`

**Implicit AND**: words without an operator default to AND:
- `"reinforcement learning"` is equivalent to `"reinforcement and learning"`

#### Combine multiple fields

Multiple `-k-*` flags are joined with AND by default:

```bash
# Title contains "diffusion model" AND author contains "ho" and "song"
arxs search -t "diffusion model" -a "ho and song"
```

Use `--op or` to switch to OR:

```bash
# Title contains "RLHF" OR abstract contains "reward model"
arxs search -t "RLHF" -b "reward model" --op or
```

## Subject Categories (-s)

`-s` supports arXiv codes and discipline aliases. Multiple `-s` flags use OR logic.

### Quick Aliases

| Alias | Domain | Primary Sources |
|-------|--------|----------------|
| `cs` | Computer Science | arXiv › Zenodo › SocArXiv |
| `physics` | All Physics | arXiv › Zenodo |
| `math` | Mathematics | arXiv › Zenodo |
| `stat` | Statistics | arXiv › Zenodo |
| `q-fin` | Quantitative Finance | arXiv › OpenAlex › Zenodo |
| `econ` | Economics | arXiv › OpenAlex › Zenodo |
| `q-bio` | Quantitative Biology | arXiv › Zenodo |
| `eess` | Electrical Engineering | arXiv › Zenodo |
| `sociology` | Sociology | SocArXiv › OpenAlex › Zenodo |
| `education` | Education | EdArXiv › SocArXiv › Zenodo |
| `philosophy` | Philosophy | OpenAlex › SocArXiv › Zenodo |
| `law` | Law | SocArXiv › OpenAlex › Zenodo |
| `psychology` | Psychology | SocArXiv › EdArXiv › Zenodo |

### arXiv Codes
Also accepts any arXiv category code: `cs.AI`, `cs.LG`, `cs.CL`, `hep-th`, `quant-ph`, `math.AG`, etc.

### Examples

```bash
arxs search -k "transformer" -s cs.AI
arxs search -k "inequality" -s sociology -s law
arxs search -k "option pricing" -s q-fin --recent 12m
arxs search -k "curriculum" -s education -s psychology
```

#### Filter by date

```bash
# Date range
arxs search -k "LLM" --from 2024-01 --to 2025-03

# Last 12 months
arxs search -k "LLM" --recent 12m

# Last 1 year
arxs search -k "LLM" --recent 1y
```

> `--recent` and `--from`/`--to` are mutually exclusive.

#### Other options

```bash
# Return up to 100 results (default 50, max 2000)
arxs search -k "GAN" --max 100

# Sort by relevance (default: submitted date descending)
arxs search -k "GAN" --sort relevance

# Custom output file
arxs search -k "GAN" -o ./gan-papers.json

# Skip cache
arxs search -k "GAN" --no-cache
```

### list — View results

```bash
arxs list                       # List results
arxs list --verbose             # Show abstracts
arxs list -n 10                 # First 10 only
arxs list -f ./gan-papers.json  # From specific file
```

### download — Download papers

```bash
arxs download 1 3 5            # Download PDFs #1, #3, #5
arxs download --all             # Download all PDFs
arxs download --abs-only 2 4    # Save abstracts as .txt
arxs download 1 --dir ./papers  # Save to directory
arxs download 1 -f ./gan.json   # From specific file
arxs download 1 --overwrite     # Overwrite existing
```

File naming: `{arXiv_ID}.pdf` or `{arXiv_ID}.txt`

### about — Tool info

```bash
arxs about
```

## Flag Reference

### search

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--key` | `-k` | Search all fields | — |
| `--key-title` | `-t` | Search title | — |
| `--key-abs` | `-b` | Search abstract | — |
| `--key-author` | `-a` | Search author | — |
| `--subject` | `-s` | Subject category (repeatable, OR logic) | arXiv-only |
| `--list-subjects` | — | Print all valid subject codes and exit | — |
| `--op` | — | Cross-field operator | `and` |
| `--from` | — | Start date (YYYY[-MM[-DD]]) | none |
| `--to` | — | End date (YYYY[-MM[-DD]]) | none |
| `--recent` | — | Recent period (6m, 12m, 1y, 2y) | — |
| `--max` | `-m` | Max results per source (1-2000) | 50 |
| `--output` | `-o` | Output JSON file | `arxiv-results.json` |
| `--sort` | — | Sort by: submitted, updated, relevance, citations | `submitted` |
| `--order` | — | Sort direction: asc, desc | `desc` |
| `--no-cache` | — | Bypass same-day cache | false |

### list

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--file` | `-f` | Results file | `arxiv-results.json` |
| `--verbose` | `-v` | Show abstracts | false |
| `--limit` | `-n` | Show first N | all |

### download

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--file` | `-f` | Results file | `arxiv-results.json` |
| `--dir` | `-d` | Save directory | `.` |
| `--abs-only` | — | Abstracts only | false |
| `--all` | — | Download all | false |
| `--overwrite` | — | Overwrite files | false |

## Compliance

This tool fully respects [arXiv's robots.txt](https://arxiv.org/robots.txt) and crawling policies:

- **No web scraping**: uses the official arXiv API (`export.arxiv.org`) exclusively, not the website
- **Rate limit**: >= 3s between requests, hardcoded and non-configurable — the `Crawl-delay: 15` in robots.txt applies to website crawlers; this tool uses only the dedicated API endpoint (`export.arxiv.org`), which carries no crawl-delay directive
- **Same-day query caching**: avoids redundant requests; arXiv data updates once daily at UTC midnight
- **Custom User-Agent**: all requests identify as `arxs/<version>` per robots.txt best practices
- **No access to restricted paths**: the tool never touches `/search`, `/auth`, `/user`, or any path disallowed by robots.txt
- Complies with [arXiv API Terms of Use](https://arxiv.org/help/api/tou)

## Usage Rules & Etiquette

### You must follow these rules

1. **Respect the arXiv service**: arXiv is a nonprofit open-access platform maintained by the academic community and volunteers. Its existence depends on good-faith usage.
2. **Do not abuse the API**: The tool enforces a 3-second rate limit. Do not attempt to bypass or modify this limit. Excessive requests impact other users.
3. **Do not bulk-download entire categories**: Only download papers you actually intend to read. arXiv is not your local backup drive.
4. **Respect author rights**: Downloaded papers are for personal academic research only. Do not use them for unauthorized commercial purposes or redistribution.
5. **Follow arXiv Terms of Use**: See [arXiv API Terms of Use](https://arxiv.org/help/api/tou).

### If you build on this tool

1. **Keep the attribution**: arXiv API ToU requires attribution in your product (already built-in).
2. **Keep the rate limiter**: Do not remove or reduce the 3-second request interval.
3. **Identify your User-Agent**: Change the UA string in `client.go` to your own project name and contact info.
4. **Do not spoof identity**: Do not forge User-Agent headers or attempt to bypass arXiv access controls.

### Be a good citizen

> We use open academic resources — we should be good citizens of the open academic community.

- arXiv serves millions of researchers annually, funded by [donations and member institutions](https://arxiv.org/about/donate)
- If arxs helps your research, consider [donating to arXiv](https://arxiv.org/about/donate)
- Cite papers properly; respect the original authors' work
- Found issues in a paper? Use proper channels, not public attacks

## License

MIT
