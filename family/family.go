// Package family reads the family records of blueprint/families.yaml (spec 09.4) and gives each declaration of the index at most one family. A family is a pattern over names, kinds, attributes and types, with one sentence that explains all its members at depth E0.
package family

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/mathlib-agent/index"
)

// Pattern is the condition of a family. Each field that is set must match.
type Pattern struct {
	// Name is a glob over the end of the full name, on a boundary of the components. "map_{op}" matches the last component and "*.coe_mk" the last two. "*" matches any text in one component, "**" matches any text, and "{op}" matches one of the ops of the family.
	Name string `yaml:"name,omitempty"`
	// Kind is the kind of the declaration, for example theorem or instance.
	Kind string `yaml:"kind,omitempty"`
	// Attr is an attribute that the declaration must have, for example simp.
	Attr string `yaml:"attr,omitempty"`
	// TypeHead is a constant that the type or the value uses, for example FunLike.
	TypeHead string `yaml:"typeHead,omitempty"`
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

	name *regexp.Regexp
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
		usesOp := strings.Contains(p.Name+f.Text, "{op}")
		if usesOp && len(f.Ops) == 0 {
			probs = append(probs, where+": the pattern uses {op} but the family has no ops")
		}
		var err error
		if f.name, err = glob(p.Name, f.Ops); err != nil {
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
	b.WriteString(`^(.*\.)?`)
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
		{"Div", "Sub"}, {"div", "sub"}, {"prod", "sum"}, {"Pow", "SMul"}, {"pow", "nsmul"},
		{"zpow", "zsmul"}, {"Monoid", "AddMonoid"}, {"Group", "AddGroup"}, {"Semigroup", "AddSemigroup"},
		{"Units", "AddUnits"}, {"Submonoid", "AddSubmonoid"}, {"Subgroup", "AddSubgroup"}, {"Mon", "AddMon"},
		{"mulIndicator", "indicator"}, {"MulAction", "AddAction"}, {"Commute", "AddCommute"},
	}
	dual = [][2]string{
		{"sup", "inf"}, {"Sup", "Inf"}, {"bot", "top"}, {"Bot", "Top"}, {"min", "max"}, {"Min", "Max"},
		{"Iic", "Ici"}, {"Iio", "Ioi"}, {"lower", "upper"},
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

// The symbol swaps of the twins in a pretty-printed type. The first symbol is the source, the second is the twin.
var (
	additiveSyms = [][2]string{{"*", "+"}, {"1", "0"}, {"/", "-"}, {"∏", "∑"}}
	dualSyms     = [][2]string{{"⊔", "⊓"}, {"⊥", "⊤"}, {"⨆", "⨅"}}
	identRe      = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_'.]*`)
)

// back changes a type of a twin to the type that its source must have: each word of each identifier and each symbol goes back to its source.
func back(t string, swaps, syms [][2]string) string {
	t = identRe.ReplaceAllStringFunc(t, func(id string) string {
		if s := sources(id, swaps); len(s) > 0 {
			return s[0]
		}
		return id
	})
	r := make([]string, 0, 2*len(syms))
	for _, s := range syms {
		r = append(r, s[1], s[0])
	}
	return strings.NewReplacer(r...).Replace(t)
}

// Twins finds the declarations whose name is the twin of another declaration of the index, for each kind of twin. The value is the name of the source. A twin must have the type of its source after the swaps, so a pair of names such as divByMonic_zero and divByMonic_one with different statements is not a twin.
func Twins(ix *index.Index) map[string]map[string]string {
	types := make(map[string]string, len(ix.Decls))
	for i := range ix.Decls {
		types[ix.Decls[i].Name] = ix.Decls[i].Type
	}
	kinds := []struct {
		name        string
		swaps, syms [][2]string
	}{{"to_additive", additive, additiveSyms}, {"to_dual", dual, dualSyms}}
	out := map[string]map[string]string{"to_additive": {}, "to_dual": {}}
	for i := range ix.Decls {
		d := &ix.Decls[i]
		for _, k := range kinds {
			for _, src := range sources(d.Name, k.swaps) {
				st, ok := types[src]
				if !ok || src == d.Name {
					continue
				}
				if back(d.Type, k.swaps, k.syms) == back(st, k.swaps, k.syms) {
					out[k.name][d.Name] = src
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
	if p.TypeHead != "" && !slices.Contains(d.Uses, p.TypeHead) {
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
