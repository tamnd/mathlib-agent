// Package rename rewrites the deprecated and moved names of Mathlib in the handbook, the companion and the E1 records (spec 05.4). It takes the rename table from the drift records between two commits. It changes Lean names only: the names in code spans, in decl directives, in Lean code blocks, in the front matter and in Lean files. It does not touch the prose.
package rename

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/informal"
)

// The kinds of an entry of the table.
const (
	Decl   = "decl"
	Module = "module"
	Move   = "move"
)

// Entry is one rename. For a move, Old is the declaration, From is its old module and New is its new module.
type Entry struct {
	Old    string `json:"old"`
	New    string `json:"new"`
	From   string `json:"from,omitempty"`
	Kind   string `json:"kind"`
	Commit string `json:"commit"`
	Note   string `json:"note,omitempty"`
}

// LoadDrift reads the drift records records/drift/*.json of the map.
func LoadDrift(mapDir string) ([]drift.Record, error) {
	paths, err := filepath.Glob(filepath.Join(mapDir, "records", "drift", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []drift.Record
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			return nil, err
		}
		var r drift.Record
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// same is true when a commit matches a short or a full form of another.
func same(a, b string) bool {
	if len(a) < 7 || len(b) < 7 {
		return a == b
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// Chain gives the drift records that lead from the commit from to the commit to, in order.
func Chain(rs []drift.Record, from, to string) ([]drift.Record, error) {
	var out []drift.Record
	cur := from
	for !same(cur, to) {
		found := false
		for _, r := range rs {
			if same(r.From, cur) {
				out = append(out, r)
				cur, found = r.To, true
				break
			}
		}
		if !found || len(out) > len(rs) {
			return nil, fmt.Errorf("no chain of drift records leads from %s to %s; the last commit of the chain is %s", from, to, cur)
		}
	}
	return out, nil
}

// Table gives the rename table of a chain. A later rename of a new name joins the earlier one, so that a → b and b → c give a → c. A rename that a type hash match proposes, with no alias, is left out unless proposals is true, because a person must accept it first.
func Table(chain []drift.Record, proposals bool) []Entry {
	var out []Entry
	idx := map[string]int{}
	for _, r := range chain {
		for _, it := range r.Items {
			var e Entry
			switch it.Class {
			case drift.Renamed:
				if strings.HasPrefix(it.Note, "same type, no alias") && !proposals {
					continue
				}
				e = Entry{Old: it.Name, New: it.To, Kind: Decl, Commit: r.To, Note: it.Note}
			case drift.ModuleRenamed:
				e = Entry{Old: it.Name, New: it.To, Kind: Module, Commit: r.To, Note: it.Note}
			case drift.Moved:
				e = Entry{Old: it.Name, New: it.To, From: it.Module, Kind: Move, Commit: r.To}
			default:
				continue
			}
			if e.Old == "" || e.New == "" || e.Old == e.New {
				continue
			}
			joined := false
			for i := range out {
				if out[i].Kind == e.Kind && out[i].New == e.Old && e.Kind != Move {
					out[i].New, out[i].Commit = e.New, e.Commit
					joined = true
				}
			}
			if k := e.Kind + " " + e.Old; !joined {
				if i, ok := idx[k]; ok {
					out[i] = e
				} else {
					idx[k] = len(out)
					out = append(out, e)
				}
			}
		}
	}
	return out
}

// Change is one rewrite in a file.
type Change struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// identRune is true for a rune that can be part of a Lean name.
func identRune(r rune) bool {
	return r == '_' || r == '.' || r == '\'' || r == '!' || r == '?' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.No, r) || r >= 0x2080 && r <= 0x209C
}

// replaceNames replaces each whole Lean name of the table in s. A name is whole when the runes on both sides cannot be part of a name.
func replaceNames(s string, names map[string]string) (string, [][2]string) {
	if len(names) == 0 {
		return s, nil
	}
	var b strings.Builder
	var done [][2]string
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !identRune(r) || r == '.' {
			b.WriteString(s[i : i+size])
			i += size
			continue
		}
		j := i
		for j < len(s) {
			r2, n := utf8.DecodeRuneInString(s[j:])
			if !identRune(r2) {
				break
			}
			j += n
		}
		tok := s[i:j]
		trail := ""
		// A name at the end of a sentence has a dot after it that is not part of the name.
		if strings.HasSuffix(tok, ".") {
			tok, trail = strings.TrimSuffix(tok, "."), "."
		}
		if n, ok := names[tok]; ok && (i == 0 || !identBefore(s[:i])) {
			b.WriteString(n)
			done = append(done, [2]string{tok, n})
		} else {
			b.WriteString(tok)
		}
		b.WriteString(trail)
		i = j
	}
	return b.String(), done
}

func identBefore(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(s)
	return identRune(r) && r != '.'
}

func maps(t []Entry) (decls, mods map[string]string, moves []Entry) {
	decls, mods = map[string]string{}, map[string]string{}
	for _, e := range t {
		switch e.Kind {
		case Decl:
			decls[e.Old] = e.New
		case Module:
			mods[e.Old] = e.New
		case Move:
			moves = append(moves, e)
		}
	}
	return decls, mods, moves
}

var (
	codeSpan  = regexp.MustCompile("`[^`\n]+`")
	directive = regexp.MustCompile(`(<\?decl\s+)(\S+?)(\s*\?>)`)
	fence     = regexp.MustCompile("^\\s*(```+|~~~+)\\s*(\\S*)")
)

// Markdown rewrites a section file of the handbook. It changes the front matter, the code spans that are one Lean name, the decl directives and the Lean code blocks. A move adds the new module to the modules list of a section that names the declaration and lists its old module.
func Markdown(file, src string, t []Entry) (string, []Change) {
	decls, mods, moves := maps(t)
	all := map[string]string{}
	for k, v := range decls {
		all[k] = v
	}
	for k, v := range mods {
		all[k] = v
	}
	lines := strings.Split(src, "\n")
	var ch []Change
	note := func(n int, done [][2]string) {
		for _, d := range done {
			ch = append(ch, Change{File: file, Line: n + 1, Old: d[0], New: d[1]})
		}
	}
	front := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
	end := -1
	inFence, lean := false, false
	for i := range lines {
		l := lines[i]
		switch {
		case front && i > 0 && end < 0:
			if strings.TrimSpace(l) == "---" {
				end = i
				continue
			}
			if strings.HasPrefix(l, "title:") || strings.HasPrefix(l, "reads_with:") {
				continue
			}
			var done [][2]string
			lines[i], done = replaceNames(l, all)
			note(i, done)
		case fence.MatchString(l):
			m := fence.FindStringSubmatch(l)
			if !inFence {
				inFence, lean = true, m[2] == "lean" || m[2] == "lean4"
			} else {
				inFence = false
			}
		case inFence:
			if lean {
				var done [][2]string
				lines[i], done = replaceNames(l, decls)
				note(i, done)
			}
		default:
			l = directive.ReplaceAllStringFunc(l, func(s string) string {
				m := directive.FindStringSubmatch(s)
				if n, ok := all[m[2]]; ok {
					note(i, [][2]string{{m[2], n}})
					return m[1] + n + m[3]
				}
				return s
			})
			l = codeSpan.ReplaceAllStringFunc(l, func(s string) string {
				inner := strings.TrimSpace(s[1 : len(s)-1])
				if n, ok := all[inner]; ok {
					note(i, [][2]string{{inner, n}})
					return "`" + n + "`"
				}
				return s
			})
			lines[i] = l
		}
	}
	if end > 0 && len(moves) > 0 {
		lines, ch = addMoves(file, lines, end, moves, ch)
	}
	return strings.Join(lines, "\n"), ch
}

// addMoves adds the new module of a moved declaration to the modules list of the front matter, when the front matter names the declaration and lists its old module.
func addMoves(file string, lines []string, end int, moves []Entry, ch []Change) ([]string, []Change) {
	head := strings.Join(lines[:end], "\n")
	for _, m := range moves {
		named, _ := replaceNames(head, map[string]string{m.Old: "\x00"})
		if !strings.Contains(named, "\x00") || strings.Contains(head, m.New) {
			continue
		}
		for i := 1; i < end; i++ {
			l := lines[i]
			t := strings.TrimSpace(l)
			switch {
			case t == "- "+m.From:
				indent := l[:len(l)-len(strings.TrimLeft(l, " "))]
				lines = append(lines[:i+1], append([]string{indent + "- " + m.New}, lines[i+1:]...)...)
				end++
			case strings.HasPrefix(t, "modules:") && strings.Contains(t, "["):
				l2, done := replaceNames(l, map[string]string{m.From: m.From + ", " + m.New})
				if len(done) == 0 {
					continue
				}
				lines[i] = l2
			default:
				continue
			}
			ch = append(ch, Change{File: file, Line: i + 1, Old: m.From, New: m.From + " and " + m.New})
			head = strings.Join(lines[:end], "\n")
			break
		}
	}
	return lines, ch
}

var importLine = regexp.MustCompile(`^(\s*(?:public\s+|meta\s+)*import\s+)(\S+)(.*)$`)

// Lean rewrites a Lean file of the companion: the modules of the import lines and the declaration names of the rest.
func Lean(file, src string, t []Entry) (string, []Change) {
	decls, mods, _ := maps(t)
	lines := strings.Split(src, "\n")
	var ch []Change
	for i, l := range lines {
		if m := importLine.FindStringSubmatch(l); m != nil {
			if n, ok := mods[m[2]]; ok {
				lines[i] = m[1] + n + m[3]
				ch = append(ch, Change{File: file, Line: i + 1, Old: m[2], New: n})
			}
			continue
		}
		var done [][2]string
		lines[i], done = replaceNames(l, decls)
		for _, d := range done {
			ch = append(ch, Change{File: file, Line: i + 1, Old: d[0], New: d[1]})
		}
	}
	return strings.Join(lines, "\n"), ch
}

// Informal renames the E1 records. The old name goes to the formerly list of the record. A record whose new name has a record already is left as it is, and the change list says so.
func Informal(file string, rs []informal.Record, t []Entry) ([]informal.Record, []Change) {
	decls, _, _ := maps(t)
	have := map[string]bool{}
	for _, r := range rs {
		have[r.Name] = true
	}
	var ch []Change
	for i := range rs {
		n, ok := decls[rs[i].Name]
		if !ok {
			continue
		}
		if have[n] {
			ch = append(ch, Change{File: file, Old: rs[i].Name, New: n + " (not changed: the new name has a record)"})
			continue
		}
		ch = append(ch, Change{File: file, Old: rs[i].Name, New: n})
		rs[i].Formerly = append(rs[i].Formerly, rs[i].Name)
		rs[i].Name = n
		have[n] = true
	}
	return rs, ch
}
