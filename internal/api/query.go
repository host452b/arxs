package api

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/host452b/arxs/internal/parser"
)

// QueryParams holds the user's search parameters.
type QueryParams struct {
	Terms     map[string]string // "title" → expr, "author" → expr, "abs" → expr, "all" → expr
	Subjects  []string          // e.g. ["cs", "math"]
	Op        string            // "and" or "or" (between -k-* fields)
	From      string            // YYYY[-MM[-DD]]
	To        string            // YYYY[-MM[-DD]]
	Max       int
	Start     int
	SortBy    string // "relevance", "submitted", "updated"
	SortOrder string // "asc", "desc"
}

var fieldMap = map[string]string{
	"title":  "ti",
	"author": "au",
	"abs":    "abs",
}

var sortByMap = map[string]string{
	"relevance": "relevance",
	"submitted": "submittedDate",
	"updated":   "lastUpdatedDate",
}

var sortOrderMap = map[string]string{
	"asc":  "ascending",
	"desc": "descending",
}

// physicsCategories are the top-level arXiv categories under physics.
var physicsCategories = []string{
	"physics", "astro-ph", "cond-mat", "gr-qc",
	"hep-ex", "hep-lat", "hep-ph", "hep-th",
	"math-ph", "nlin", "nucl-ex", "nucl-th", "quant-ph",
}

// BuildQueryURL constructs the full arXiv API query URL.
func BuildQueryURL(p QueryParams) string {
	var parts []string

	// Build search term parts
	termParts := buildTermParts(p.Terms, p.Op)
	if len(termParts) > 0 {
		parts = append(parts, termParts)
	}

	// Build subject filter
	subjectPart := buildSubjectPart(p.Subjects)
	if subjectPart != "" {
		parts = append(parts, subjectPart)
	}

	// Build date filter
	datePart := buildDatePart(p.From, p.To)
	if datePart != "" {
		parts = append(parts, datePart)
	}

	searchQuery := strings.Join(parts, " AND ")

	// Build URL
	vals := url.Values{}
	vals.Set("search_query", searchQuery)
	vals.Set("start", fmt.Sprintf("%d", p.Start))
	vals.Set("max_results", fmt.Sprintf("%d", p.Max))

	if sb, ok := sortByMap[p.SortBy]; ok {
		vals.Set("sortBy", sb)
	}
	if so, ok := sortOrderMap[p.SortOrder]; ok {
		vals.Set("sortOrder", so)
	}

	return "https://export.arxiv.org/api/query?" + vals.Encode()
}

func buildTermParts(terms map[string]string, op string) string {
	if len(terms) == 0 {
		return ""
	}

	if op == "" {
		op = "and"
	}

	var exprs []string

	// Sort keys for deterministic output
	keys := make([]string, 0, len(terms))
	for k := range terms {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		expr := terms[key]
		if key == "all" {
			exprs = append(exprs, parser.ParseAllFields(expr))
		} else if field, ok := fieldMap[key]; ok {
			exprs = append(exprs, parser.ParseExpr(expr, field))
		}
	}

	return parser.CombineExprs(exprs, op)
}

func buildSubjectPart(subjects []string) string {
	if len(subjects) == 0 {
		return ""
	}

	var catParts []string
	for _, s := range subjects {
		if s == "physics" {
			for _, pc := range physicsCategories {
				catParts = append(catParts, "cat:"+pc+".*")
			}
		} else {
			catParts = append(catParts, "cat:"+s+".*")
		}
	}

	if len(catParts) == 1 {
		return catParts[0]
	}
	return "(" + strings.Join(catParts, " OR ") + ")"
}

func buildDatePart(from, to string) string {
	if from == "" && to == "" {
		return ""
	}

	fromDate := "000001010000"
	toDate := "999912312359"

	if from != "" {
		fromDate = normalizeDateStart(from)
	}
	if to != "" {
		toDate = normalizeDateEnd(to)
	}

	return "submittedDate:[" + fromDate + " TO " + toDate + "]"
}

// normalizeDateStart converts "2024" → "202401010000", "2024-01" → "202401010000", "2024-01-15" → "202401150000"
func normalizeDateStart(d string) string {
	d = strings.ReplaceAll(d, "-", "")
	switch len(d) {
	case 4: // YYYY
		return d + "01010000"
	case 6: // YYYYMM
		return d + "010000"
	case 8: // YYYYMMDD
		return d + "0000"
	}
	return d
}

// normalizeDateEnd converts "2024" → "202412312359", "2024-01" → "202401312359", "2024-01-15" → "202401152359"
func normalizeDateEnd(d string) string {
	d = strings.ReplaceAll(d, "-", "")
	switch len(d) {
	case 4: // YYYY
		return d + "12312359"
	case 6: // YYYYMM
		return d + "312359" // Simplified: use 31 for all months
	case 8: // YYYYMMDD
		return d + "2359"
	}
	return d
}
