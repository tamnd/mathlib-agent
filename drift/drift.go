// Package drift compares two indexes of Mathlib and writes the drift items of spec 05.4.
package drift

import (
	"sort"
	"strings"

	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/index"
)

// The classes of spec 05.4 that the index can find. The classes visibility and silent come from the build, and curated from the curated lists.
const (
	Added         = "added"
	Removed       = "removed"
	Renamed       = "renamed"
	Deprecated    = "deprecated"
	TypeChanged   = "type-changed"
	ValueChanged  = "value-changed"
	AttrChanged   = "attr-changed"
	DocChanged    = "doc-changed"
	Moved         = "moved"
	ModuleAdded   = "module-added"
	ModuleRemoved = "module-removed"
	ModuleRenamed = "module-renamed"
	Curated       = "curated"
)

// The severities of spec 05.4.
const (
	Info     = "info"
	Minor    = "minor"
	Breaking = "breaking"
	Rename   = "rename"
	Move     = "move"
)

// Item is one drift item.
type Item struct {
	Class    string `json:"class"`
	Name     string `json:"name"`
	To       string `json:"to,omitempty"` // the new name or the new module
	Module   string `json:"module,omitempty"`
	Severity string `json:"severity"`
	Note     string `json:"note,omitempty"`
}

// Record is the file records/drift/<date>.json.
type Record struct {
	Date      string         `json:"date"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Toolchain [2]string      `json:"toolchain"`
	Counts    map[string]int `json:"counts"`
	Items     []Item         `json:"items"`
}

// Options set what Compare reports.
type Options struct {
	Depths map[string]depth.Result // the depths at the new commit, for the severity of a type change
	Lists  [2]depth.Lists          // the curated lists at the old and the new commit, or nil
	// Added lists each added declaration as an item. Without it the record gives only the count, which keeps the record small for a first sync.
	Added bool
}

// Compare returns the drift from old to new. When the toolchains differ, the type hashes cannot be compared, and the type text is used instead.
func Compare(old, cur *index.Index, o Options) Record {
	r := Record{
		From:      old.Header.Mathlib,
		To:        cur.Header.Mathlib,
		Toolchain: [2]string{old.Header.Toolchain, cur.Header.Toolchain},
		Counts:    map[string]int{},
	}
	sameHash := old.Header.Toolchain == cur.Header.Toolchain
	add := func(it Item) {
		r.Counts[it.Class]++
		if it.Class == Added && !o.Added {
			return
		}
		r.Items = append(r.Items, it)
	}
	typeKey := func(d *index.Decl) string {
		if sameHash {
			return d.TypeHash
		}
		return d.Type
	}
	// The old names that are gone, by type, to find renames with no alias.
	added := map[string][]*index.Decl{}
	for i := range cur.Decls {
		d := &cur.Decls[i]
		if _, ok := old.Lookup(d.Name); !ok {
			added[typeKey(d)] = append(added[typeKey(d)], d)
		}
	}
	targets := map[string]bool{}
	for i := range cur.Decls {
		if dep := cur.Decls[i].Deprecated; dep != nil && dep.Target != "" {
			targets[dep.Target] = true
		}
	}
	renamedTo := map[string]bool{}
	for i := range old.Decls {
		od := &old.Decls[i]
		nd, ok := cur.Lookup(od.Name)
		if !ok {
			if cands := added[typeKey(od)]; len(cands) > 0 {
				best := nearest(od, cands)
				renamedTo[best.Name] = true
				add(Item{Class: Renamed, Name: od.Name, To: best.Name, Module: best.Module, Severity: Rename, Note: "same type, no alias: a proposal for a person to accept"})
				continue
			}
			add(Item{Class: Removed, Name: od.Name, Module: od.Module, Severity: Info})
			continue
		}
		if nd.Deprecated != nil && od.Deprecated == nil {
			if nd.Deprecated.Target != "" {
				add(Item{Class: Renamed, Name: od.Name, To: nd.Deprecated.Target, Module: nd.Module, Severity: Rename, Note: "deprecated alias since " + nd.Deprecated.Since})
			} else {
				add(Item{Class: Deprecated, Name: od.Name, Module: nd.Module, Severity: Info, Note: "deprecated since " + nd.Deprecated.Since})
			}
			continue
		}
		if nd.Deprecated != nil {
			continue
		}
		if typeKey(od) != typeKey(nd) {
			sev := Minor
			if dr, ok := o.Depths[nd.Name]; ok && (dr.Depth == depth.E3 || dr.Depth == depth.E2) {
				sev = Breaking
			}
			add(Item{Class: TypeChanged, Name: nd.Name, Module: nd.Module, Severity: sev})
		} else if sameHash && od.ValueHash != nd.ValueHash && od.ValueHash != "" {
			add(Item{Class: ValueChanged, Name: nd.Name, Module: nd.Module, Severity: Minor})
		}
		if od.Module != nd.Module {
			add(Item{Class: Moved, Name: nd.Name, To: nd.Module, Module: od.Module, Severity: Move})
		}
		if a, b := attrs(od), attrs(nd); a != b {
			add(Item{Class: AttrChanged, Name: nd.Name, Module: nd.Module, Severity: Minor, Note: a + " to " + b})
		}
		if od.Doc != nd.Doc {
			add(Item{Class: DocChanged, Name: nd.Name, Module: nd.Module, Severity: Info})
		}
	}
	for i := range cur.Decls {
		nd := &cur.Decls[i]
		if _, ok := old.Lookup(nd.Name); ok || renamedTo[nd.Name] || nd.Deprecated != nil {
			continue
		}
		add(Item{Class: Added, Name: nd.Name, Module: nd.Module, Severity: Info})
	}
	om, nm := moduleSet(old), moduleSet(cur)
	for m := range om {
		if !nm[m] {
			add(Item{Class: ModuleRemoved, Name: m, Severity: Info})
		}
	}
	for m := range nm {
		if !om[m] {
			add(Item{Class: ModuleAdded, Name: m, Severity: Info})
		}
	}
	if o.Lists[0] != nil && o.Lists[1] != nil {
		for n := range o.Lists[0] {
			if _, ok := o.Lists[1][n]; !ok {
				add(Item{Class: Curated, Name: n, Severity: Info, Note: "left the curated lists"})
			}
		}
		for n, l := range o.Lists[1] {
			if _, ok := o.Lists[0][n]; !ok {
				add(Item{Class: Curated, Name: n, Severity: Info, Note: "added to " + strings.Join(l, ", ")})
			}
		}
	}
	sort.SliceStable(r.Items, func(i, j int) bool {
		if r.Items[i].Class != r.Items[j].Class {
			return r.Items[i].Class < r.Items[j].Class
		}
		return r.Items[i].Name < r.Items[j].Name
	})
	return r
}

// nearest picks the candidate whose module shares the longest prefix with the module of d.
func nearest(d *index.Decl, cands []*index.Decl) *index.Decl {
	best, score := cands[0], -1
	for _, c := range cands {
		s := commonPrefix(d.Module, c.Module)
		if s > score || (s == score && c.Name < best.Name) {
			best, score = c, s
		}
	}
	return best
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// attrs returns the reader-facing attributes, sorted, in one string.
func attrs(d *index.Decl) string {
	var xs []string
	for _, a := range d.Attrs {
		if a != "deprecated" {
			xs = append(xs, a)
		}
	}
	sort.Strings(xs)
	if len(xs) == 0 {
		return "none"
	}
	return strings.Join(xs, ",")
}

func moduleSet(ix *index.Index) map[string]bool {
	out := map[string]bool{}
	for _, m := range ix.Modules {
		out[m.Module] = true
	}
	return out
}
