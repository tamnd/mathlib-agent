// Package coverage computes the coverage numbers of spec 04.5 and the module records of spec 09.1.
package coverage

import (
	"sort"

	"github.com/tamnd/mathlib-agent/internal/blueprint"
	"github.com/tamnd/mathlib-agent/internal/depth"
	"github.com/tamnd/mathlib-agent/internal/index"
)

// Counts are the declarations of a module or a volume by depth.
type Counts struct {
	Decls int            `json:"decls"`
	Depth map[string]int `json:"depth"`
	Kind  map[string]int `json:"kind,omitempty"`
}

func newCounts() Counts { return Counts{Depth: map[string]int{}, Kind: map[string]int{}} }

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

// Compute returns the coverage record and the module rows. No section of the handbook exists yet, so C-home and C-depth are zero until the section plans and the front matter give them (spec 07.7).
func Compute(ix *index.Index, vm *blueprint.VolumeMap, depths map[string]depth.Result, date string) (Record, []ModuleRow) {
	mods := map[string]*ModuleRow{}
	vols := map[string]*VolumeRow{}
	for _, v := range vm.Volumes {
		vols[v.ID] = &VolumeRow{Volume: v.ID, Title: v.Title, Counts: newCounts()}
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
		if v, ok := vols[row.Volume]; ok {
			v.add(d, r)
		} else {
			rec.Orphans++
		}
	}
	for _, v := range vm.Volumes {
		row := vols[v.ID]
		if row.Depth[depth.E3] > 0 {
			zero := 0.0
			row.CE3 = &zero
		}
		rec.Volumes = append(rec.Volumes, *row)
	}
	if rec.Total.Depth[depth.E3] > 0 {
		zero := 0.0
		rec.Total.CE3 = &zero
	}
	rows := make([]ModuleRow, 0, len(mods))
	for _, r := range mods {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Module < rows[j].Module })
	return rec, rows
}
