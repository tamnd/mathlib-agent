// Package coverage computes the coverage numbers of spec 04.5 and the module records of spec 09.1.
package coverage

import (
	"sort"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/index"
)

// Counts are the declarations of a module or a volume by depth.
type Counts struct {
	Decls int            `json:"decls"`
	Depth map[string]int `json:"depth"`
	Kind  map[string]int `json:"kind,omitempty"`
}

func newCounts() Counts { return Counts{Depth: map[string]int{}, Kind: map[string]int{}} }

// tally counts the declarations that the book gives a home, that it explains at the required depth, and the E3 candidates that it explains at E3.
type tally struct{ home, deep, e3, e3done int }

func (t *tally) add(b *Book, d *index.Decl, r depth.Result) {
	if b == nil {
		return
	}
	dp, ok := b.treatment(d.Name, d.Module)
	if r.Depth == depth.E3 {
		t.e3++
	}
	if !ok {
		return
	}
	if len(b.Homes[d.Module]) > 0 || b.Treated[d.Name] != "" {
		t.home++
	}
	if rank[dp] >= rank[r.Depth] {
		t.deep++
		if r.Depth == depth.E3 {
			t.e3done++
		}
	}
}

func (t tally) set(v *VolumeRow) {
	if v.Decls > 0 {
		v.CHome = float64(t.home) / float64(v.Decls)
		v.CDepth = float64(t.deep) / float64(v.Decls)
	}
	if t.e3 > 0 {
		c := float64(t.e3done) / float64(t.e3)
		v.CE3 = &c
	}
}

func (c *Counts) add(d *index.Decl, r depth.Result) {
	c.Decls++
	c.Depth[r.Depth]++
	c.Kind[d.Kind]++
}

// ModuleRow is one row of records/modules.json.
type ModuleRow struct {
	Module   string   `json:"module"`
	Volume   string   `json:"volume"`
	Sections []string `json:"sections"`
	Counts
}

// VolumeRow is one row of the coverage of records/coverage.json.
type VolumeRow struct {
	Volume  string   `json:"volume"`
	Title   string   `json:"title"`
	Modules int      `json:"modules"`
	CHome   float64  `json:"cHome"`  // the share of declarations with a home section
	CDepth  float64  `json:"cDepth"` // the share explained at the required depth or higher
	CE3     *float64 `json:"cE3"`    // the E3 candidates with an E3 treatment; null when there are none
	Counts
}

// Record is records/coverage.json.
type Record struct {
	Mathlib   string      `json:"mathlib"`
	Toolchain string      `json:"toolchain"`
	Date      string      `json:"date"`
	Total     VolumeRow   `json:"total"`
	Volumes   []VolumeRow `json:"volumes"`
	// Orphans are the declarations whose module has no home volume. The volume map must make it zero.
	Orphans int `json:"orphans"`
	// Aliases are the deprecated aliases. They do not count as declarations (spec 04.6).
	Aliases int `json:"aliases"`
}

// Compute returns the coverage record and the module rows with no book, so C-home, C-depth and C-E3 are zero.
func Compute(ix *index.Index, vm *blueprint.VolumeMap, depths map[string]depth.Result, date string) (Record, []ModuleRow) {
	return ComputeBook(ix, vm, depths, nil, date)
}

// ComputeBook returns the coverage record and the module rows, with C-home, C-depth and C-E3 from the front matter of the book (spec 04.5 and 07.7). A nil book gives zero.
func ComputeBook(ix *index.Index, vm *blueprint.VolumeMap, depths map[string]depth.Result, book *Book, date string) (Record, []ModuleRow) {
	tallies := map[string]*tally{}
	var all tally
	mods := map[string]*ModuleRow{}
	vols := map[string]*VolumeRow{}
	for _, v := range vm.Volumes {
		vols[v.ID] = &VolumeRow{Volume: v.ID, Title: v.Title, Counts: newCounts()}
		tallies[v.ID] = &tally{}
	}
	rec := Record{Mathlib: ix.Header.Mathlib, Toolchain: ix.Header.Toolchain, Date: date}
	rec.Total = VolumeRow{Volume: "all", Title: "Mathlib", Counts: newCounts()}
	for _, m := range ix.Modules {
		vol, _, _ := vm.Home(index.ModulePath(m.Module))
		mods[m.Module] = &ModuleRow{Module: m.Module, Volume: vol, Sections: []string{}, Counts: newCounts()}
		if v, ok := vols[vol]; ok {
			v.Modules++
		}
		rec.Total.Modules++
	}
	for i := range ix.Decls {
		d := &ix.Decls[i]
		if d.Deprecated != nil {
			rec.Aliases++
			continue
		}
		r := depths[d.Name]
		row, ok := mods[d.Module]
		if !ok {
			vol, _, _ := vm.Home(d.Path())
			row = &ModuleRow{Module: d.Module, Volume: vol, Sections: []string{}, Counts: newCounts()}
			mods[d.Module] = row
		}
		row.add(d, r)
		rec.Total.add(d, r)
		all.add(book, d, r)
		if v, ok := vols[row.Volume]; ok {
			v.add(d, r)
			tallies[row.Volume].add(book, d, r)
		} else {
			rec.Orphans++
		}
	}
	for _, v := range vm.Volumes {
		row := vols[v.ID]
		tallies[v.ID].set(row)
		if row.CE3 == nil && row.Depth[depth.E3] > 0 {
			zero := 0.0
			row.CE3 = &zero
		}
		rec.Volumes = append(rec.Volumes, *row)
	}
	all.set(&rec.Total)
	if rec.Total.CE3 == nil && rec.Total.Depth[depth.E3] > 0 {
		zero := 0.0
		rec.Total.CE3 = &zero
	}
	rows := make([]ModuleRow, 0, len(mods))
	for _, r := range mods {
		if book != nil && len(book.Homes[r.Module]) > 0 {
			r.Sections = book.Homes[r.Module]
		}
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Module < rows[j].Module })
	return rec, rows
}
