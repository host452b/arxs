# arxs

A CLI tool to search and download arXiv papers. Uses the official arXiv API with built-in rate limiting and caching.

> Thank you to arXiv for use of its open access interoperability.

[中文文档](README_CN.md)

## Install

**One-liner (macOS / Linux):**

```bash
curl -fsSL https://raw.githubusercontent.com/host452b/arxs/main/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/host452b/arxs/main/install.ps1 | iex
```

**Go install:**

```bash
go install github.com/host452b/arxs@latest
```

**Build from source:**

```bash
git clone https://github.com/host452b/arxs.git
cd arxs
go build -o arxs .
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

#### Filter by subject

Supported: `cs`, `math`, `physics`, `q-bio`, `q-fin`, `stat`, `eess`, `econ`

```bash
# Computer science only
arxs search -k "LLM" -s cs

# Computer science and statistics
arxs search -k "machine learning" -s cs,stat
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
| `--subject` | `-s` | Subject categories | all |
| `--op` | — | Cross-field operator | `and` |
| `--from` | — | Start date | none |
| `--to` | — | End date | none |
| `--recent` | — | Recent period | — |
| `--max` | `-m` | Max results | 50 |
| `--output` | `-o` | Output file | `arxiv-results.json` |
| `--sort` | — | Sort by | `submitted` |
| `--order` | — | Sort direction | `desc` |
| `--no-cache` | — | Skip cache | false |

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
- **Rate limit**: >= 3s between requests, hardcoded and non-configurable — exceeds the robots.txt `Crawl-delay: 15` requirement since we use the dedicated API endpoint, not the website
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
