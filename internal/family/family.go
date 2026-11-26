// Package family reads the family records of blueprint/families.yaml (spec 09.4) and gives each declaration of the index at most one family. A family is a pattern over names, kinds, attributes and types, with one sentence that explains all its members at depth E0.
package family

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/mathlib-agent/internal/index"
)

// Pattern is the condition of a family. Each field that is set must match.
type Pattern struct {
	// Name is a glob over the full name. "*" matches any text in one name component, "**" matches any text, and "{op}" matches one of the ops of the family.
	Name string `yaml:"name,omitempty"`
	// Last is a glob over the last component of the name only.
	Last string `yaml:"last,omitempty"`
	// Kind is the kind of the declaration, for example theorem or instance.
	Kind string `yaml:"kind,omitempty"`
	// Attr is an attribute that the declaration must have, for example simp.
	Attr string `yaml:"attr,omitempty"`
	// TypeHas is a text that the pretty-printed type must contain.
	TypeHas string `yaml:"typeHas,omitempty"`
	// Twin is "to_additive" or "to_dual". The declaration matches when its name is the twin of another declaration of the index.
	Twin string `yaml:"twin,omitempty"`
	// Deprecated matches the deprecated aliases.
	Deprecated bool `yaml:"deprecated,omitempty"`
}

// Family is one record of blueprint/families.yaml.
type Family struct {
	ID      string   `yaml:"id"`
	Pattern Pattern  `yaml:"pattern"`
	Ops     []string `yaml:"ops,omitempty"`
	Text    string   `yaml:"text"`
	Depth   string   `yaml:"depth"`

	name, last *regexp.Regexp
}

// Load reads and compiles blueprint/families.yaml.
func Load(path string) ([]*Family, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var fs []*Family
	if err := yaml.Unmarshal(b, &fs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if probs := Check(fs); len(probs) > 0 {
		return nil, fmt.Errorf("%s: %s", path, strings.Join(probs, "; "))
	}
	return fs, nil
}

var idRe = regexp.MustCompile(`^F-[a-z0-9-]+$`)

// Check finds the faults of the records and compiles the patterns: an ID of a wrong form or two times, no text, a depth that is not E0, a pattern with no condition, an {op} with no ops, and an unknown twin.
func Check(fs []*Family) []string {
	var probs []string
	seen := map[string]bool{}
	for i, f := range fs {
		where := fmt.Sprintf("family %d (%s)", i+1, f.ID)
		if !idRe.MatchString(f.ID) {
			probs = append(probs, where+": the id must be of the form F-name")
		}
		if seen[f.ID] {
			probs = append(probs, where+": the id is used two times")
		}
		seen[f.ID] = true
		if strings.TrimSpace(f.Text) == "" {
			probs = append(probs, where+": the family has no text")
		}
		if f.Depth != "E0" {
			probs = append(probs, where+": the depth of a family is E0")
		}
		p := f.Pattern
		if p == (Pattern{}) {
			probs = append(probs, where+": the pattern has no condition")
		}
		if p.Twin != "" && p.Twin != "to_additive" && p.Twin != "to_dual" {
			probs = append(probs, where+": the twin must be to_additive or to_dual")
		}
		usesOp := strings.Contains(p.Name+p.Last+f.Text, "{op}")
		if usesOp && len(f.Ops) == 0 {
			probs = append(probs, where+": the pattern uses {op} but the family has no ops")
		}
		var err error
		if f.name, err = glob(p.Name, f.Ops); err != nil {
			probs = append(probs, where+": "+err.Error())
		}
		if f.last, err = glob(p.Last, f.Ops); err != nil {
			probs = append(probs, where+": "+err.Error())
		}
	}
	return probs
}

// glob compiles a name glob to an anchored regular expression.
func glob(g string, ops []string) (*regexp.Regexp, error) {
	if g == "" {
		return nil, nil
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); {
		switch {
		case strings.HasPrefix(g[i:], "**"):
			b.WriteString(".*")
			i += 2
		case g[i] == '*':
			b.WriteString(`[^.]*`)
			i++
		case strings.HasPrefix(g[i:], "{op}"):
			qs := make([]string, len(ops))
			for j, o := range ops {
				qs[j] = regexp.QuoteMeta(o)
			}
			b.WriteString("(" + strings.Join(qs, "|") + ")")
			i += 4
		default:
			b.WriteString(regexp.QuoteMeta(g[i : i+1]))
			i++
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// The word swaps of the twins. The first word is the source, the second is the twin.
var (
	additive = [][2]string{
		{"Mul", "Add"}, {"mul", "add"}, {"One", "Zero"}, {"one", "zero"}, {"Inv", "Neg"}, {"inv", "neg"},
		{"Div", "Sub"}, {"div", "sub"}, {"Prod", "Sum"}, {"prod", "sum"}, {"Pow", "SMul"}, {"pow", "nsmul"},
		{"zpow", "zsmul"}, {"Monoid", "AddMonoid"}, {"Group", "AddGroup"}, {"Semigroup", "AddSemigroup"},
		{"Units", "AddUnits"}, {"Submonoid", "AddSubmonoid"}, {"Subgroup", "AddSubgroup"}, {"Mon", "AddMon"},
		{"mulIndicator", "indicator"}, {"MulAction", "AddAction"}, {"Commute", "AddCommute"},
	}
	dual = [][2]string{
		{"sup", "inf"}, {"Sup", "Inf"}, {"bot", "top"}, {"Bot", "Top"}, {"min", "max"}, {"Min", "Max"},
		{"le", "ge"}, {"lt", "gt"}, {"left", "right"}, {"Iic", "Ici"}, {"Iio", "Ioi"}, {"lower", "upper"},
		{"Lower", "Upper"}, {"monotone", "antitone"}, {"Monotone", "Antitone"},
	}
	splitRe = regexp.MustCompile(`[A-Z]?[a-z0-9]+|[A-Z]+(?:[A-Z][a-z]|$)|[^A-Za-z0-9]`)
)

// sources gives the names that come from n when each word of a swap list goes back to its source. The words are the parts of each name component in lower camel case or upper camel case.
func sources(n string, swaps [][2]string) []string {
	parts := splitRe.FindAllString(n, -1)
	if strings.Join(parts, "") != n {
		return nil
	}
	back := map[string]string{}
	for _, s := range swaps {
		back[s[1]] = s[0]
	}
	var out []string
	changed := false
	cur := make([]string, len(parts))
	for i, p := range parts {
		if src, ok := back[p]; ok {
			cur[i] = src
			changed = true
		} else {
			cur[i] = p
		}
	}
	if changed {
		out = append(out, strings.Join(cur, ""))
	}
	return out
}

// Twins finds the declarations whose name is the twin of another declaration of the index, for each kind of twin. The value is the name of the source.
func Twins(ix *index.Index) map[string]map[string]string {
	names := make(map[string]bool, len(ix.Decls))
	for i := range ix.Decls {
		names[ix.Decls[i].Name] = true
	}
	out := map[string]map[string]string{"to_additive": {}, "to_dual": {}}
	for i := range ix.Decls {
		n := ix.Decls[i].Name
		for kind, swaps := range map[string][][2]string{"to_additive": additive, "to_dual": dual} {
			for _, src := range sources(n, swaps) {
				if src != n && names[src] {
					out[kind][n] = src
				}
			}
		}
	}
	return out
}

// Result is the outcome of Match.
type Result struct {
	// Member gives the family of each declaration that has one.
	Member map[string]string
	// Count gives the number of members of each family.
	Count map[string]int
	// Twin gives the source of each twin member.
	Twin map[string]string
}

func lastComponent(n string) string {
	if i := strings.LastIndexByte(n, '.'); i >= 0 {
		return n[i+1:]
	}
	return n
}

// Matches tells whether one declaration matches the pattern of a family. twins comes from Twins.
func (f *Family) Matches(d *index.Decl, twins map[string]map[string]string) bool {
	p := f.Pattern
	if p.Deprecated != (d.Deprecated != nil) {
		return false
	}
	if p.Kind != "" && d.Kind != p.Kind {
		return false
	}
	if p.Attr != "" && !d.Has(p.Attr) {
		return false
	}
	if p.TypeHas != "" && !strings.Contains(d.Type, p.TypeHas) {
		return false
	}
	if f.name != nil && !f.name.MatchString(d.Name) {
		return false
	}
	if f.last != nil && !f.last.MatchString(lastComponent(d.Name)) {
		return false
	}
	if p.Twin != "" {
		if _, ok := twins[p.Twin][d.Name]; !ok {
			return false
		}
	}
	return true
}

// Match gives each declaration of the index the first family whose pattern it matches. A declaration that matches no family has none.
func Match(ix *index.Index, fs []*Family) Result {
	twins := Twins(ix)
	r := Result{Member: map[string]string{}, Count: map[string]int{}, Twin: map[string]string{}}
	for _, f := range fs {
		r.Count[f.ID] = 0
	}
	for i := range ix.Decls {
		d := &ix.Decls[i]
		for _, f := range fs {
			if f.Matches(d, twins) {
				r.Member[d.Name] = f.ID
				r.Count[f.ID]++
				if f.Pattern.Twin != "" {
					r.Twin[d.Name] = twins[f.Pattern.Twin][d.Name]
				}
				break
			}
		}
	}
	return r
}

// Summary is one row of records/families.json.
type Summary struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Members int      `json:"members"`
	Sample  []string `json:"sample"`
}

// Record is records/families.json.
type Record struct {
	Mathlib  string    `json:"mathlib"`
	Date     string    `json:"date"`
	Decls    int       `json:"declarations"`
	Members  int       `json:"members"`
	Families []Summary `json:"families"`
}

// Summarize gives the record of a match, with up to n sample members of each family in name order.
func Summarize(ix *index.Index, fs []*Family, r Result, date string, n int) Record {
	samples := map[string][]string{}
	for name, id := range r.Member {
		samples[id] = append(samples[id], name)
	}
	rec := Record{Mathlib: ix.Header.Mathlib, Date: date, Decls: len(ix.Decls), Members: len(r.Member)}
	for _, f := range fs {
		s := samples[f.ID]
		sort.Strings(s)
		if len(s) > n {
			s = s[:n]
		}
		if s == nil {
			s = []string{}
		}
		rec.Families = append(rec.Families, Summary{ID: f.ID, Text: f.Text, Members: r.Count[f.ID], Sample: s})
	}
	return rec
}
