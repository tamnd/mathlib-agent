// Package stale finds the sections of the handbook that a drift makes stale, and writes the stale marks of spec 05.5.
//
// A section is stale when a drift record after the mathlib commit of the section has a breaking or a minor item about a declaration of its explains list, a name in its text, or a module of its modules list. A rename or a move item does not make a section stale, because mlib rename -apply fixes it. The mark sets status: stale and stale_since in the front matter, and stale_from keeps the status before the mark, so that the refresher can set it back (spec 07.8).
package stale

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/mathlib-agent/drift"
)

// Section is the part of a section file that the marks read.
type Section struct {
	ID       string
	Path     string
	Modules  []string
	Explains []string
	Names    []string // the names of the text: the decl directives and the code spans that look like a name
	Mathlib  string
	Status   string
	From     string // stale_from, when the section is already stale
	Since    string // stale_since, when the section is already stale
}

// Hit is one drift item of a stale section, with the commit at which it happened.
type Hit struct {
	drift.Item
	Commit string `json:"commit"`
	Date   string `json:"date"`
	Why    string `json:"why"`
}

// Mark is the refresher queue item of a stale section.
type Mark struct {
	Section string `json:"section"`
	Path    string `json:"path"`
	Mathlib string `json:"mathlib"`
	Since   string `json:"staleSince"`
	From    string `json:"staleFrom"`
	Items   []Hit  `json:"items"`
}

// Breaking is true when a person must accept the change of the refresher (spec 05.5).
func (m Mark) Breaking() bool {
	for _, h := range m.Items {
		if h.Severity == drift.Breaking {
			return true
		}
	}
	return false
}

// Record is the file records/stale.json: the refresher queue items at a commit.
type Record struct {
	Mathlib string `json:"mathlib"`
	Date    string `json:"date"`
	Marks   []Mark `json:"marks"`
}

// Path is the place of the record in a checkout of tamnd/mathlib.
func Path(mapDir string) string {
	return filepath.Join(mapDir, "records", "stale.json")
}

// Load reads the record. A missing file gives an empty record.
func Load(path string) (*Record, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return &Record{}, nil
	}
	if err != nil {
		return nil, err
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &r, nil
}

// ID gives the id of a section from its path: content/13-x/05-y/02-z.md gives V13.C05.S02. A file that is not a section gives "".
func ID(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) < 3 {
		return ""
	}
	parts = parts[len(parts)-3:]
	var n [3]int
	for i, p := range parts {
		if len(p) < 3 || p[2] != '-' && p[2] != '.' {
			return ""
		}
		v, err := strconv.Atoi(p[:2])
		if err != nil {
			return ""
		}
		n[i] = v
	}
	if !strings.HasSuffix(parts[2], ".md") {
		return ""
	}
	return fmt.Sprintf("V%02d.C%02d.S%02d", n[0], n[1], n[2])
}

// front splits a file into its front matter and the rest. ok is false when the file has no front matter.
func front(src string) (fm, body string, start, end int, ok bool) {
	if !strings.HasPrefix(src, "---\n") {
		return "", src, 0, 0, false
	}
	i := strings.Index(src[4:], "\n---\n")
	if i < 0 {
		return "", src, 0, 0, false
	}
	return src[4 : 4+i+1], src[4+i+5:], 4, 4 + i + 1, true
}

var (
	declRE = regexp.MustCompile(`<\?decl\s+([^\s?]+)`)
	spanRE = regexp.MustCompile("`([^`\n]+)`")
	nameRE = regexp.MustCompile(`^[\pL_][\pL\pN_'!?₀-₉]*(\.[\pL_][\pL\pN_'!?₀-₉]*)*$`)
)

// Parse reads a section file. ok is false when the file has no front matter with a status.
func Parse(path, src string) (Section, bool, error) {
	fm, body, _, _, ok := front(src)
	if !ok {
		return Section{}, false, nil
	}
	var f struct {
		Modules  []string       `yaml:"modules"`
		Explains map[string]any `yaml:"explains"`
		Mathlib  string         `yaml:"mathlib"`
		Status   string         `yaml:"status"`
		From     string         `yaml:"stale_from"`
		Since    string         `yaml:"stale_since"`
	}
	if err := yaml.Unmarshal([]byte(fm), &f); err != nil {
		return Section{}, false, fmt.Errorf("%s: %w", path, err)
	}
	if f.Status == "" {
		return Section{}, false, nil
	}
	s := Section{ID: ID(path), Path: path, Modules: f.Modules, Mathlib: f.Mathlib, Status: f.Status, From: f.From, Since: f.Since}
	for _, v := range f.Explains {
		if l, ok := v.([]any); ok { // E1: auto is not a list
			for _, n := range l {
				s.Explains = append(s.Explains, fmt.Sprint(n))
			}
		}
	}
	sort.Strings(s.Explains)
	seen := map[string]bool{}
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			s.Names = append(s.Names, n)
		}
	}
	for _, m := range declRE.FindAllStringSubmatch(body, -1) {
		add(m[1])
	}
	for _, m := range spanRE.FindAllStringSubmatch(body, -1) {
		if nameRE.MatchString(m[1]) {
			add(m[1])
		}
	}
	sort.Strings(s.Names)
	return s, true, nil
}

// Find gives the mark of a section from the drift records after its commit, in order. ok is false when no item makes the section stale.
func Find(s Section, chain []drift.Record) (Mark, bool) {
	explains, names, mods := set(s.Explains), set(s.Names), set(s.Modules)
	m := Mark{Section: s.ID, Path: s.Path, Mathlib: s.Mathlib, Since: s.Since, From: s.From}
	for _, r := range chain {
		for _, it := range r.Items {
			if it.Severity != drift.Breaking && it.Severity != drift.Minor {
				continue
			}
			why := ""
			switch {
			case explains[it.Name]:
				why = "the section explains " + it.Name
			case names[it.Name]:
				why = "the text uses " + it.Name
			case it.Module != "" && mods[it.Module]:
				why = "the section takes the module " + it.Module
			case (it.Class == drift.ModuleRemoved || it.Class == drift.ModuleAdded) && mods[it.Name]:
				why = "the section takes the module " + it.Name
			default:
				continue
			}
			m.Items = append(m.Items, Hit{Item: it, Commit: r.To, Date: r.Date, Why: why})
			if m.Since == "" {
				m.Since = r.To
			}
		}
	}
	if len(m.Items) == 0 {
		return Mark{}, false
	}
	if m.From == "" {
		m.From = s.Status
	}
	return m, true
}

func set(ss []string) map[string]bool {
	out := make(map[string]bool, len(ss))
	for _, s := range ss {
		out[s] = true
	}
	return out
}

// Apply writes the mark into the front matter of a section file: status: stale, stale_since and stale_from. A section that is already stale keeps its first stale_since and its stale_from.
func Apply(src string, m Mark) (string, error) {
	fm, _, start, end, ok := front(src)
	if !ok {
		return "", errors.New("the file has no front matter")
	}
	var out []string
	done := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimSuffix(fm, "\n"), "\n") {
		key, _, _ := strings.Cut(line, ":")
		switch key {
		case "status":
			out = append(out, "status: stale")
			done[key] = true
		case "stale_since", "stale_from":
			continue // written once after status
		default:
			out = append(out, line)
			continue
		}
		out = append(out, "stale_since: "+m.Since, "stale_from: "+m.From)
	}
	if !done["status"] {
		return "", errors.New("the front matter has no status")
	}
	return src[:start] + strings.Join(out, "\n") + "\n" + src[end:], nil
}
