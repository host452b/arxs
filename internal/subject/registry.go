// internal/subject/registry.go
package subject

import (
	"fmt"
	"sort"
	"strings"

	"github.com/host452b/arxs/v2/internal/provider"
)

// LookupResult holds the ordered provider list and merged filters for a set of subjects.
type LookupResult struct {
	Providers []provider.ProviderID
	Filter    provider.SubjectFilter
}

// entry defines mapping for one subject code or alias.
type entry struct {
	providers        []provider.ProviderID
	arxivCats        []string
	openAlexConcepts []string
	zenodoKeywords   []string
	osfProviders     []string
	osfSubjects      []string
}

// registry maps subject codes and aliases to their entry.
var registry = map[string]entry{
	// ── Computer Science ──────────────────────────────────────────────
	"cs":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs"}, openAlexConcepts: []string{"C41008148"}, zenodoKeywords: []string{"computer science"}},
	"cs.ai": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.AI"}, openAlexConcepts: []string{"C154945302"}, zenodoKeywords: []string{"artificial intelligence"}},
	"cs.lg": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.LG"}, openAlexConcepts: []string{"C119857082"}, zenodoKeywords: []string{"machine learning"}},
	"cs.cl": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.CL"}, openAlexConcepts: []string{"C204321447"}, zenodoKeywords: []string{"natural language processing"}},
	"cs.cv": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.CV"}, openAlexConcepts: []string{"C31972630"}, zenodoKeywords: []string{"computer vision"}},
	"cs.cr": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.CR"}, openAlexConcepts: []string{"C38652104"}, zenodoKeywords: []string{"cybersecurity"}},
	"cs.ro": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.RO"}, openAlexConcepts: []string{"C11413529"}, zenodoKeywords: []string{"robotics"}},
	"cs.cy": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderSocArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.CY"}, openAlexConcepts: []string{"C17744445"}, zenodoKeywords: []string{"computers and society"}, osfProviders: []string{"socarxiv"}, osfSubjects: []string{"Social and Behavioral Sciences"}},
	"cs.hc": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo, provider.ProviderSocArxiv}, arxivCats: []string{"cs.HC"}, openAlexConcepts: []string{"C121332964"}, zenodoKeywords: []string{"human-computer interaction"}},
	"cs.se": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.SE"}, zenodoKeywords: []string{"software engineering"}},
	"cs.dc": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.DC"}, zenodoKeywords: []string{"distributed computing"}},
	"cs.ni": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.NI"}, zenodoKeywords: []string{"computer networking"}},
	"cs.gt": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"cs.GT"}, openAlexConcepts: []string{"C2993651"}, zenodoKeywords: []string{"game theory"}},
	"cs.ir": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cs.IR"}, zenodoKeywords: []string{"information retrieval"}},

	// ── Physics ───────────────────────────────────────────────────────
	"physics":  {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"physics", "astro-ph", "cond-mat", "gr-qc", "hep-ex", "hep-lat", "hep-ph", "hep-th", "math-ph", "nlin", "nucl-ex", "nucl-th", "quant-ph"}, zenodoKeywords: []string{"physics"}},
	"hep-th":   {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"hep-th"}, zenodoKeywords: []string{"high energy physics theoretical"}},
	"hep-ex":   {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"hep-ex"}, zenodoKeywords: []string{"high energy physics experimental"}},
	"hep-ph":   {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"hep-ph"}, zenodoKeywords: []string{"phenomenology"}},
	"hep-lat":  {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"hep-lat"}, zenodoKeywords: []string{"lattice QCD"}},
	"quant-ph": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"quant-ph"}, openAlexConcepts: []string{"C62520636"}, zenodoKeywords: []string{"quantum computing"}},
	"cond-mat": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"cond-mat"}, zenodoKeywords: []string{"condensed matter"}},
	"astro-ph": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"astro-ph"}, zenodoKeywords: []string{"astrophysics"}},
	"gr-qc":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"gr-qc"}, zenodoKeywords: []string{"general relativity"}},
	"nucl-ex":  {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"nucl-ex"}, zenodoKeywords: []string{"nuclear physics"}},
	"nucl-th":  {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"nucl-th"}, zenodoKeywords: []string{"nuclear theory"}},
	"math-ph":  {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math-ph"}, zenodoKeywords: []string{"mathematical physics"}},
	"nlin":     {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"nlin"}, zenodoKeywords: []string{"nonlinear science"}},

	// ── Mathematics ───────────────────────────────────────────────────
	"math":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math"}, openAlexConcepts: []string{"C33923547"}, zenodoKeywords: []string{"mathematics"}},
	"math.ag": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.AG"}, zenodoKeywords: []string{"algebraic geometry"}},
	"math.nt": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.NT"}, zenodoKeywords: []string{"number theory"}},
	"math.pr": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.PR"}, zenodoKeywords: []string{"probability"}},
	"math.co": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.CO"}, zenodoKeywords: []string{"combinatorics"}},
	"math.na": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"math.NA"}, zenodoKeywords: []string{"numerical analysis"}},
	"math.lo": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"math.LO"}, openAlexConcepts: []string{"C138885662"}, zenodoKeywords: []string{"mathematical logic"}},

	// ── Statistics ────────────────────────────────────────────────────
	"stat":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"stat"}, openAlexConcepts: []string{"C161191863"}, zenodoKeywords: []string{"statistics"}},
	"stat.ml": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"stat.ML"}, zenodoKeywords: []string{"statistical machine learning"}},

	// ── Quantitative Finance & Economics ──────────────────────────────
	"q-fin":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"q-fin"}, openAlexConcepts: []string{"C187279774"}, zenodoKeywords: []string{"quantitative finance"}},
	"q-fin.tr": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"q-fin.TR"}, zenodoKeywords: []string{"algorithmic trading"}},
	"q-fin.rm": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"q-fin.RM"}, zenodoKeywords: []string{"risk management"}},
	"econ":     {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"econ"}, openAlexConcepts: []string{"C162324750"}, zenodoKeywords: []string{"economics"}},
	"econ.em":  {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, arxivCats: []string{"econ.EM"}, zenodoKeywords: []string{"econometrics"}},

	// ── Quantitative Biology & EESS ───────────────────────────────────
	"q-bio":   {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"q-bio"}, openAlexConcepts: []string{"C86803240"}, zenodoKeywords: []string{"quantitative biology"}},
	"eess":    {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"eess"}, openAlexConcepts: []string{"C41008148"}, zenodoKeywords: []string{"electrical engineering"}},
	"eess.sp": {providers: []provider.ProviderID{provider.ProviderArxiv, provider.ProviderZenodo}, arxivCats: []string{"eess.SP"}, zenodoKeywords: []string{"signal processing"}},

	// ── Social Sciences ───────────────────────────────────────────────
	"sociology":  {providers: []provider.ProviderID{provider.ProviderSocArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, openAlexConcepts: []string{"C144024400"}, zenodoKeywords: []string{"sociology"}, osfProviders: []string{"socarxiv"}, osfSubjects: []string{"Social and Behavioral Sciences"}},
	"law":        {providers: []provider.ProviderID{provider.ProviderSocArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, openAlexConcepts: []string{"C18214049"}, zenodoKeywords: []string{"law"}, osfProviders: []string{"socarxiv"}, osfSubjects: []string{"Law"}},
	"psychology": {providers: []provider.ProviderID{provider.ProviderSocArxiv, provider.ProviderEdArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C15744967"}, zenodoKeywords: []string{"psychology"}, osfProviders: []string{"socarxiv"}, osfSubjects: []string{"Social and Behavioral Sciences"}},
	"political":  {providers: []provider.ProviderID{provider.ProviderSocArxiv, provider.ProviderOpenAlex, provider.ProviderZenodo}, openAlexConcepts: []string{"C17744445"}, zenodoKeywords: []string{"political science"}, osfProviders: []string{"socarxiv"}, osfSubjects: []string{"Political Science"}},
	"economics":  {providers: []provider.ProviderID{provider.ProviderOpenAlex, provider.ProviderArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C162324750"}, zenodoKeywords: []string{"economics"}},
	"management": {providers: []provider.ProviderID{provider.ProviderOpenAlex, provider.ProviderSocArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C144133560"}, zenodoKeywords: []string{"management"}},

	// ── Education ─────────────────────────────────────────────────────
	"education": {providers: []provider.ProviderID{provider.ProviderEdArxiv, provider.ProviderSocArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C142362112"}, zenodoKeywords: []string{"education"}, osfProviders: []string{"edarxiv"}, osfSubjects: []string{"Education"}},

	// ── Philosophy ────────────────────────────────────────────────────
	"philosophy": {providers: []provider.ProviderID{provider.ProviderOpenAlex, provider.ProviderSocArxiv, provider.ProviderZenodo}, arxivCats: []string{"physics.hist-ph"}, openAlexConcepts: []string{"C138885662"}, zenodoKeywords: []string{"philosophy"}, osfProviders: []string{"socarxiv"}, osfSubjects: []string{"Philosophy"}},
	"ethics":     {providers: []provider.ProviderID{provider.ProviderOpenAlex, provider.ProviderSocArxiv, provider.ProviderZenodo}, openAlexConcepts: []string{"C119599485"}, zenodoKeywords: []string{"ethics"}},
}

// ErrUnknownSubject is returned when a subject code or alias is not in the registry.
type ErrUnknownSubject struct {
	Subject string
}

func (e ErrUnknownSubject) Error() string {
	return fmt.Sprintf("unknown subject %q — run 'arxs search --list-subjects' for valid values", e.Subject)
}

// Lookup resolves a list of subject strings into a merged LookupResult.
// Each string is lowercased before lookup. Comma-separated entries are split.
// Returns ErrUnknownSubject if any entry is not found.
func Lookup(subjects []string) (*LookupResult, error) {
	// Expand comma-separated entries
	var expanded []string
	for _, s := range subjects {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part != "" {
				expanded = append(expanded, part)
			}
		}
	}

	if len(expanded) == 0 {
		return nil, fmt.Errorf("no subjects provided")
	}

	// Validate all subjects first
	for _, s := range expanded {
		if _, ok := registry[s]; !ok {
			return nil, ErrUnknownSubject{Subject: s}
		}
	}

	// Merge entries
	providerSeen := map[provider.ProviderID]bool{}
	var providers []provider.ProviderID
	filter := provider.SubjectFilter{}

	for _, s := range expanded {
		e := registry[s]
		for _, p := range e.providers {
			if !providerSeen[p] {
				providerSeen[p] = true
				providers = append(providers, p)
			}
		}
		filter.ArxivCats = unionStrings(filter.ArxivCats, e.arxivCats)
		filter.OpenAlexConcepts = unionStrings(filter.OpenAlexConcepts, e.openAlexConcepts)
		filter.ZenodoKeywords = unionStrings(filter.ZenodoKeywords, e.zenodoKeywords)
		filter.OSFSubjects = unionStrings(filter.OSFSubjects, e.osfSubjects)
		filter.OSFProviders = unionStrings(filter.OSFProviders, e.osfProviders)
	}

	return &LookupResult{Providers: providers, Filter: filter}, nil
}

// ValidSubjects returns a sorted list of all known subject codes and aliases.
func ValidSubjects() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	for _, s := range a {
		seen[s] = true
	}
	result := append([]string{}, a...)
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
