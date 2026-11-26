// Package plan makes the first section plan of a volume (spec 09.1, 07.7) from the index, the volume map, the depths and the families. The plan is a draft for the planner role and for people: it gives the chapters, the modules of each section, the E3 and E2 lists, the families and the requires edges. A person accepts the plan.
package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/index"
)

// The page estimates of spec 04.7.
const (
	PagesE3  = 0.8
	PagesE2  = 0.15
	RowsPage = 30
)

// Section is one section of the plan.
type Section struct {
	ID       string         `yaml:"id"`
	Title    string         `yaml:"title"`
	Kind     string         `yaml:"kind"`
	Modules  []string       `yaml:"modules"`
	E3       []string       `yaml:"E3,omitempty,flow"`
	E2       []string       `yaml:"E2,omitempty,flow"`
	Families []string       `yaml:"families,omitempty,flow"`
	Requires []string       `yaml:"requires,omitempty,flow"`
	Counts   map[string]int `yaml:"counts,flow"`
	Pages    float64        `yaml:"pages"`
	Status   string         `yaml:"status"`
}

// Chapter is one chapter of the plan.
type Chapter struct {
	ID       string    `yaml:"id"`
	Title    string    `yaml:"title"`
	Prefixes []string  `yaml:"prefixes,flow"`
	Requires []string  `yaml:"requires,omitempty,flow"`
	Sections []Section `yaml:"sections"`
}

// Plan is blueprint/sections/<vol>.yaml.
type Plan struct {
	Volume   string    `yaml:"volume"`
	Title    string    `yaml:"title"`
	Mathlib  string    `yaml:"mathlib"`
	Date     string    `yaml:"date"`
	By       string    `yaml:"by"`
	Status   string    `yaml:"status"`
	Pages    float64   `yaml:"pages"`
	Chapters []Chapter `yaml:"chapters"`
}

// Options are the sizes that the plan uses.
type Options struct {
	// SectionPages is the size of a section that the plan aims at.
	SectionPages float64
	// ChapterModules is the largest number of modules of a chapter. A larger prefix splits by its next folder.
	ChapterModules int
	// ChapterPages is the smallest size of a chapter. Small groups that come one after the other join into one chapter.
	ChapterPages float64
	// MinUses is the smallest number of uses that makes a requires edge.
	MinUses int
	// By names the tool that made the plan.
	By   string
	Date string
}

// Defaults are the options of mlib plan.
var Defaults = Options{SectionPages: 10, ChapterModules: 40, ChapterPages: 20, MinUses: 3}

// Pages gives the page estimate of spec 04.7 for counts by depth.
func Pages(c map[string]int) float64 {
	p := PagesE3*float64(c[depth.E3]) + PagesE2*float64(c[depth.E2]) + float64(c[depth.E1])/RowsPage
	return float64(int(p*10+0.5)) / 10
}

// group is a set of modules of one chapter before the sections.
type group struct {
	key     string
	modules []string
	keys    []string
}

// join joins the groups that come one after the other until each has at least the given pages. The pages of a group come from size.
func join(gs []group, least float64, size func(group) float64) []group {
	var out []group
	var cur *group
	var pages float64
	for _, g := range gs {
		if cur == nil {
			c := group{key: g.key, keys: []string{g.key}, modules: append([]string{}, g.modules...)}
			cur, pages = &c, size(g)
		} else {
			cur.keys = append(cur.keys, g.key)
			cur.modules = append(cur.modules, g.modules...)
			pages += size(g)
		}
		if pages >= least {
			out = append(out, *cur)
			cur = nil
		}
	}
	if cur != nil {
		if len(out) > 0 && pages < least/2 {
			last := &out[len(out)-1]
			last.keys = append(last.keys, cur.keys...)
			last.modules = append(last.modules, cur.modules...)
		} else {
			out = append(out, *cur)
		}
	}
	return out
}

// titles joins the titles of the keys of a chapter.
func titles(keys []string) string {
	var ts []string
	for _, k := range keys {
		if k == "other" {
			ts = append(ts, "other modules")
		} else {
			ts = append(ts, title(k))
		}
	}
	switch {
	case len(ts) == 1:
		return ts[0]
	case len(ts) <= 4:
		return strings.Join(ts[:len(ts)-1], ", ") + " and " + ts[len(ts)-1]
	}
	return strings.Join(ts[:3], ", ") + " and other topics"
}

// chapters splits the modules of a volume by their prefix, and a prefix with too many modules by its next folder.
func chapters(vol *blueprint.Volume, vm *blueprint.VolumeMap, mods []string, most int) []group {
	byPrefix := map[string][]string{}
	for _, m := range mods {
		v, p, ok := vm.Home(m)
		if !ok || v != vol.ID {
			continue
		}
		byPrefix[p] = append(byPrefix[p], m)
	}
	var out []group
	for _, p := range vol.Prefixes {
		ms := byPrefix[p]
		delete(byPrefix, p)
		if len(ms) == 0 {
			continue
		}
		sort.Strings(ms)
		if len(ms) <= most {
			out = append(out, group{key: p, modules: ms})
			continue
		}
		sub := map[string][]string{}
		var keys []string
		for _, m := range ms {
			k := p
			if rest, ok := strings.CutPrefix(m, p+"/"); ok {
				first, _, _ := strings.Cut(rest, "/")
				k = p + "/" + first
			}
			if _, ok := sub[k]; !ok {
				keys = append(keys, k)
			}
			sub[k] = append(sub[k], m)
		}
		// A folder with one module joins the group of the prefix, so that the chapters are not too small.
		var rest []string
		for _, k := range keys {
			if len(sub[k]) == 1 {
				rest = append(rest, sub[k]...)
				continue
			}
			out = append(out, group{key: k, modules: sub[k]})
		}
		if len(rest) > 0 {
			out = append(out, group{key: p, modules: rest})
		}
	}
	// The exceptions of the volume map that point into this volume.
	var left []string
	for _, ms := range byPrefix {
		left = append(left, ms...)
	}
	if len(left) > 0 {
		sort.Strings(left)
		out = append(out, group{key: "other", modules: left})
	}
	return out
}

// title gives a title from a module path: the last folder, with the words of an upper camel case name split.
func title(path string) string {
	last := path[strings.LastIndex(path, "/")+1:]
	var b strings.Builder
	for i, r := range last {
		if i > 0 && r >= 'A' && r <= 'Z' && (last[i-1] < 'A' || last[i-1] > 'Z') {
			b.WriteByte(' ')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// sectionTitle gives the title of a section from its modules: the title of the one module, or of the longest folder that has all the modules. When that folder is only the area, the title is that of the first module.
func sectionTitle(mods []string) string {
	if len(mods) == 1 {
		return title(strings.ReplaceAll(mods[0], ".", "/"))
	}
	parts := strings.Split(mods[0], ".")
	n := len(parts)
	for _, m := range mods[1:] {
		q := strings.Split(m, ".")
		i := 0
		for i < n && i < len(q) && parts[i] == q[i] {
			i++
		}
		n = i
	}
	if n >= 3 {
		return title(strings.Join(parts[:n], "/"))
	}
	return title(strings.ReplaceAll(mods[0], ".", "/"))
}

// dedupe gives the sections of a chapter that have the same title a longer title with their first module.
func dedupe(ss []Section) {
	n := map[string]int{}
	for _, s := range ss {
		n[s.Title]++
	}
	for i := range ss {
		if n[ss[i].Title] > 1 {
			first := title(strings.ReplaceAll(ss[i].Modules[0], ".", "/"))
			if first == ss[i].Title {
				first = "basics"
			}
			ss[i].Title += ": " + first
		}
	}
}

// Input is what Make reads.
type Input struct {
	Index   *index.Index
	Volumes *blueprint.VolumeMap
	Depths  map[string]depth.Result
	// Families gives the family of each member, from family.Match. It can be nil.
	Families map[string]string
}

// Make makes the plan of one volume.
func Make(in Input, volID string, o Options) (*Plan, error) {
	var vol *blueprint.Volume
	for i := range in.Volumes.Volumes {
		if in.Volumes.Volumes[i].ID == volID {
			vol = &in.Volumes.Volumes[i]
		}
	}
	if vol == nil {
		return nil, fmt.Errorf("the volume map has no volume %s", volID)
	}
	byModule := map[string][]*index.Decl{}
	for i := range in.Index.Decls {
		d := &in.Index.Decls[i]
		if d.Deprecated == nil {
			byModule[d.Path()] = append(byModule[d.Path()], d)
		}
	}
	var mods []string
	for i := range in.Index.Modules {
		mods = append(mods, index.ModulePath(in.Index.Modules[i].Module))
	}
	p := &Plan{Volume: vol.ID, Title: vol.Title, Mathlib: in.Index.Header.Mathlib, Date: o.Date, By: o.By, Status: "proposed"}
	// The chapter of each module, for the requires edges.
	chapterOf := map[string]string{}
	groups := chapters(vol, in.Volumes, mods, o.ChapterModules)
	keyOf := map[string]string{}
	for _, g := range groups {
		for _, m := range g.modules {
			keyOf[m] = g.key
		}
	}
	groups = join(groups, o.ChapterPages, func(g group) float64 {
		c := map[string]int{}
		for _, m := range g.modules {
			for _, d := range byModule[m] {
				c[in.Depths[d.Name].Depth]++
			}
		}
		return Pages(c)
	})
	for i, g := range groups {
		id := fmt.Sprintf("C%02d", i+1)
		for _, m := range g.modules {
			chapterOf[m] = id
		}
	}
	// The place of each declaration: the chapter in this volume, or the other volume.
	place := func(name string) string {
		d, ok := in.Index.Lookup(name)
		if !ok {
			return ""
		}
		if c, ok := chapterOf[d.Path()]; ok {
			return vol.ID + "." + c
		}
		if v, _, ok := in.Volumes.Home(d.Path()); ok {
			return v
		}
		return ""
	}
	kind := "theory"
	if vol.Level == "guide" {
		kind = "guide"
	}
	for i, g := range groups {
		ch := Chapter{ID: fmt.Sprintf("C%02d", i+1), Title: titles(g.keys)}
		for _, k := range g.keys {
			if k != "other" {
				ch.Prefixes = append(ch.Prefixes, k)
			}
		}
		ch.Title = strings.ToUpper(ch.Title[:1]) + ch.Title[1:]
		chUses := map[string]int{}
		var cur *Section
		flush := func() {
			if cur == nil {
				return
			}
			cur.Pages = Pages(cur.Counts)
			cur.Title = sectionTitle(cur.Modules)
			if cur.Kind == "theory" && cur.Counts[depth.E3]+cur.Counts[depth.E2] == 0 {
				cur.Kind = "reference"
			}
			ch.Sections = append(ch.Sections, *cur)
			cur = nil
		}
		prev := ""
		for _, m := range g.modules {
			ds := byModule[m]
			// A section ends at the border of two prefixes when it has half its size.
			if cur != nil && keyOf[m] != keyOf[prev] && Pages(cur.Counts) >= o.SectionPages/2 {
				flush()
			}
			prev = m
			if cur == nil {
				cur = &Section{ID: fmt.Sprintf("%s.%s.S%02d", vol.ID, ch.ID, len(ch.Sections)+1), Kind: kind, Counts: map[string]int{}, Status: "planned"}
			}
			cur.Modules = append(cur.Modules, strings.ReplaceAll(m, "/", "."))
			fams := map[string]bool{}
			for _, f := range cur.Families {
				fams[f] = true
			}
			uses := map[string]int{}
			for _, d := range ds {
				r := in.Depths[d.Name]
				cur.Counts[r.Depth]++
				switch r.Depth {
				case depth.E3:
					cur.E3 = append(cur.E3, d.Name)
				case depth.E2:
					cur.E2 = append(cur.E2, d.Name)
				}
				if f, ok := in.Families[d.Name]; ok && !fams[f] {
					fams[f] = true
					cur.Families = append(cur.Families, f)
				}
				if r.Depth == depth.E3 || r.Depth == depth.E2 {
					for _, u := range d.Uses {
						if pl := place(u); pl != "" && pl != vol.ID+"."+ch.ID {
							uses[pl]++
							chUses[pl]++
						}
					}
				}
			}
			cur.Requires = merge(cur.Requires, edges(uses, o.MinUses))
			if Pages(cur.Counts) >= o.SectionPages {
				flush()
			}
		}
		flush()
		// A last section much smaller than the target joins the one before it.
		if n := len(ch.Sections); n >= 2 && ch.Sections[n-1].Pages < o.SectionPages/4 {
			a, b := &ch.Sections[n-2], ch.Sections[n-1]
			a.Modules = append(a.Modules, b.Modules...)
			a.E3, a.E2 = append(a.E3, b.E3...), append(a.E2, b.E2...)
			a.Families = merge(a.Families, b.Families)
			a.Requires = merge(a.Requires, b.Requires)
			for k, v := range b.Counts {
				a.Counts[k] += v
			}
			a.Pages = Pages(a.Counts)
			a.Title = sectionTitle(a.Modules)
			if a.Kind == "reference" && b.Kind == "theory" {
				a.Kind = "theory"
			}
			ch.Sections = ch.Sections[:n-1]
		}
		dedupe(ch.Sections)
		ch.Requires = edges(chUses, o.MinUses)
		for _, s := range ch.Sections {
			p.Pages += s.Pages
		}
		p.Chapters = append(p.Chapters, ch)
	}
	p.Pages = float64(int(p.Pages*10+0.5)) / 10
	return p, nil
}

// edges gives the places with at least the given uses, in order of the place.
func edges(uses map[string]int, least int) []string {
	var out []string
	for pl, n := range uses {
		if n >= least {
			out = append(out, pl)
		}
	}
	sort.Strings(out)
	return out
}

// merge gives the union of two sorted lists, in order.
func merge(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range append(append([]string{}, a...), b...) {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

// Path is the place of the plan of a volume in a checkout of tamnd/mathlib.
func Path(mapDir, vol string) string {
	return filepath.Join(mapDir, "blueprint", "sections", vol+".yaml")
}

// Save writes the plan with a comment line on top.
func (p *Plan) Save(path string) error {
	b, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	head := fmt.Sprintf("# The section plan of %s (spec 09.1, 07.7). mlib plan made the draft from the index at %s. A person accepts it and then edits it by hand.\n", p.Volume, short(p.Mathlib))
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(head), b...), 0o600)
}

// Load reads a plan.
func Load(path string) (*Plan, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &p, nil
}

func short(c string) string {
	if len(c) > 10 {
		return c[:10]
	}
	return c
}
