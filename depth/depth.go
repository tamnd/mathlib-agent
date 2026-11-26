// Package depth computes the depth E0 to E3 of each declaration of the index with the rules of spec 04.4.
package depth

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/mathlib-agent/index"
)

// The depths of spec 04.3.
const (
	E0 = "E0"
	E1 = "E1"
	E2 = "E2"
	E3 = "E3"
)

// Depths lists the depths from the highest to the lowest.
var Depths = []string{E3, E2, E1, E0}

// Result is the computed depth of one declaration and the rule that gave it.
type Result struct {
	Depth  string `json:"depth"`
	Reason string `json:"reason"`
}

// Lists are the curated lists of Mathlib (spec 04.4, rule 1): the names in docs/overview.yaml, docs/undergrad.yaml, docs/100.yaml and docs/1000.yaml.
type Lists map[string][]string // name to the lists that have it

// CuratedFiles are the files of the curated lists, relative to a Mathlib checkout.
var CuratedFiles = []string{"docs/overview.yaml", "docs/undergrad.yaml", "docs/100.yaml", "docs/1000.yaml"}

// LoadLists reads the curated lists from a Mathlib checkout. A file that is not there is skipped. The lists are nested YAML, so the reader takes every string value and keeps the ones that look like a Lean name. Compute keeps only the names that are in the index.
func LoadLists(mathlib string) (Lists, error) {
	out := Lists{}
	for _, f := range CuratedFiles {
		b, err := os.ReadFile(filepath.Clean(filepath.Join(mathlib, f)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var root yaml.Node
		if err := yaml.Unmarshal(b, &root); err != nil {
			return nil, err
		}
		list := strings.TrimSuffix(filepath.Base(f), ".yaml")
		walk(&root, func(s string) {
			for _, n := range strings.Fields(s) {
				n = strings.Trim(n, "`'\",;")
				if leanName.MatchString(n) {
					out[n] = appendOnce(out[n], list)
				}
			}
		})
	}
	return out, nil
}

var leanName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_'!?₀-₉]*(\.[A-Za-z_][A-Za-z0-9_'!?₀-₉]*)*$`)

func walk(n *yaml.Node, f func(string)) {
	if n.Kind == yaml.ScalarNode {
		f(n.Value)
	}
	for i, c := range n.Content {
		// The keys of a mapping are titles, not names.
		if n.Kind == yaml.MappingNode && i%2 == 0 {
			continue
		}
		walk(c, f)
	}
}

func appendOnce(xs []string, x string) []string {
	for _, y := range xs {
		if y == x {
			return xs
		}
	}
	return append(xs, x)
}

// mainHeading finds the headings of a module docstring that list the main items.
var mainHeading = regexp.MustCompile(`(?im)^#+\s*(main\s+(definitions?|results?|statements?|theorems?|declarations?|constructions?))\s*$`)

var backticked = regexp.MustCompile("`([^`\\s]+)`")

// MainNames returns the names in backticks under the "Main ..." headings of the module docstrings, up to the next heading.
func MainNames(docs []string) []string {
	var out []string
	for _, doc := range docs {
		locs := mainHeading.FindAllStringIndex(doc, -1)
		for _, l := range locs {
			rest := doc[l[1]:]
			if next := regexp.MustCompile(`(?m)^#+\s`).FindStringIndex(rest); next != nil {
				rest = rest[:next[0]]
			}
			for _, m := range backticked.FindAllStringSubmatch(rest, -1) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// namedResult matches a docstring that names a result, such as "the Bolzano-Weierstrass theorem".
var namedResult = regexp.MustCompile(`\b[A-Z][a-zà-ž]+(?:[-–][A-Z][a-zà-ž]+)*(?:'s)?\s+(?:theorem|lemma|inequality|formula|criterion|identity|conjecture|principle)\b`)

// e0 name patterns of rule 3.
var (
	e0Suffix = regexp.MustCompile(`(_iff|_def|_apply|_eq|_mk|_symm_apply|_coe|_toFun|_val|Aux|_aux)$`)
	e0Prefix = regexp.MustCompile(`^(coe_|map_|mem_|toFun_|val_)`)
)

func lastComponent(n string) string {
	if i := strings.LastIndex(n, "."); i >= 0 {
		return n[i+1:]
	}
	return n
}

// Compute gives each declaration of the index its depth. topShare is the share of the classes and structures, ranked by used-by count, that are E3 candidates (rule 1, 0.02 in the spec). minUses is the used-by count that makes a theorem E2 (rule 2, 5 in the spec).
func Compute(ix *index.Index, lists Lists, topShare float64, minUses int) map[string]Result {
	usedBy := ix.UsedBy()
	main := map[string]string{}
	for _, m := range ix.Modules {
		for _, n := range MainNames(m.Docs) {
			main[n] = m.Module
		}
	}
	// Rank the classes and structures by used-by count.
	var types []string
	for i := range ix.Decls {
		if k := ix.Decls[i].Kind; k == "class" || k == "structure" {
			types = append(types, ix.Decls[i].Name)
		}
	}
	sort.SliceStable(types, func(i, j int) bool { return usedBy[types[i]] > usedBy[types[j]] })
	top := map[string]bool{}
	for _, t := range types[:int(float64(len(types))*topShare)] {
		top[t] = true
	}
	out := make(map[string]Result, len(ix.Decls))
	for i := range ix.Decls {
		d := &ix.Decls[i]
		out[d.Name] = rule(d, lists, main, top, usedBy, minUses)
	}
	// A name in a main heading can be relative to the namespace of the module. Match the short name against the declarations of the module.
	short := map[string]map[string]bool{}
	for n, mod := range main {
		if short[mod] == nil {
			short[mod] = map[string]bool{}
		}
		short[mod][n] = true
	}
	for i := range ix.Decls {
		d := &ix.Decls[i]
		if out[d.Name].Depth == E3 || d.Deprecated != nil {
			continue
		}
		s := short[d.Module]
		if s == nil {
			continue
		}
		for n := range s {
			if strings.HasSuffix(d.Name, "."+n) {
				out[d.Name] = Result{E3, "named under a Main heading of its module"}
				break
			}
		}
	}
	return out
}

func rule(d *index.Decl, lists Lists, main map[string]string, top map[string]bool, usedBy map[string]int, minUses int) Result {
	if d.Deprecated != nil {
		return Result{E0, "a deprecated alias, listed under its target"}
	}
	// Rule 1.
	if l, ok := lists[d.Name]; ok {
		return Result{E3, "in the curated list " + strings.Join(l, ", ")}
	}
	if _, ok := main[d.Name]; ok {
		return Result{E3, "named under a Main heading of its module"}
	}
	if top[d.Name] {
		return Result{E3, "a class or structure in the top of the used-by ranking"}
	}
	// Rule 3 comes before rule 2 for the mechanical forms: a simp lemma used five times is still a simp lemma.
	last := lastComponent(d.Name)
	switch {
	case d.Kind == "instance":
		return Result{E0, "an instance"}
	case d.Kind == "ctor":
		return Result{E0, "a constructor, explained with its type"}
	case e0Suffix.MatchString(last) || e0Prefix.MatchString(last):
		return Result{E0, "a name of a family pattern"}
	case d.Has("simp") && d.Kind == "theorem":
		return Result{E0, "a simp lemma"}
	}
	// Rule 2.
	if d.Kind == "theorem" {
		if usedBy[d.Name] >= minUses {
			return Result{E2, "used by other modules"}
		}
		if namedResult.MatchString(d.Doc) {
			return Result{E2, "its docstring names a result"}
		}
	}
	return Result{E1, "no rule above"}
}
