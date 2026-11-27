package agent

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/index"
	"github.com/tamnd/mathlib-agent/informal"
	"github.com/tamnd/mathlib-agent/plan"
	"github.com/tamnd/mathlib-agent/stale"
)

// MinGroup is the least number of declarations in a group of the family role.
const MinGroup = 5

// Input is what the queues read.
type Input struct {
	Index      *index.Index
	Volumes    *blueprint.VolumeMap
	Milestones []blueprint.Milestone
	// Current is the id of the current milestone.
	Current string
	Depths  map[string]depth.Result
	// Families maps the name of a member to its family.
	Families map[string]string
	// Plans and Informal are keyed by the volume id.
	Plans    map[string]*plan.Plan
	Informal map[string][]informal.Record
	Runs     []*Run
	Date     time.Time
	// Stale maps the id of a stale section to its refresher queue item, from records/stale.json.
	Stale map[string]stale.Mark
}

// Item is one item of the queue of a role.
type Item struct {
	Role   string  `json:"role"`
	Target string  `json:"target"`
	Kind   string  `json:"kind"`
	Title  string  `json:"title"`
	Volume string  `json:"volume,omitempty"`
	Rank   float64 `json:"rank"`
	Weight float64 `json:"weight"`
	// Readers is the number of planned sections whose requires include the chapter or the volume of the item.
	Readers int     `json:"readers"`
	Level   float64 `json:"level"`
	Failed  int     `json:"failed"`
	Age     float64 `json:"age"`
	// Size is the number of declarations that the item has to treat. It breaks ties.
	Size   int      `json:"size"`
	Why    []string `json:"why"`
	Names  []string `json:"-"`
	Module string   `json:"-"`
	Plan   *plan.Plan
	Chap   *plan.Chapter
	Sect   *plan.Section
	order  int
}

// Reason gives the parts of the rank in words.
func (it Item) Reason() string {
	return strings.Join(it.Why, "; ")
}

// e1 gives the declarations at depth E1 that are not deprecated, by home volume and module.
func (in *Input) e1() map[string]map[string][]*index.Decl {
	out := map[string]map[string][]*index.Decl{}
	for i := range in.Index.Decls {
		d := &in.Index.Decls[i]
		if d.Deprecated != nil || in.Depths[d.Name].Depth != depth.E1 {
			continue
		}
		v, _, ok := in.Volumes.Home(d.Path())
		if !ok {
			continue
		}
		if out[v] == nil {
			out[v] = map[string][]*index.Decl{}
		}
		out[v][d.Module] = append(out[v][d.Module], d)
	}
	return out
}

// fresh gives the names with a record whose type hash matches the index.
func (in *Input) fresh(vol string) map[string]bool {
	out := map[string]bool{}
	for _, r := range in.Informal[vol] {
		if d, ok := in.Index.Lookup(r.Name); ok && d.TypeHash == r.TypeHash {
			out[r.Name] = true
		}
	}
	return out
}

// readers counts the planned sections whose requires include the key, or a key that starts with the key and a dot.
func (in *Input) readers(key string) int {
	n := 0
	for _, p := range in.Plans {
		for _, c := range p.Chapters {
			for _, s := range c.Sections {
				if s.Status != "planned" {
					continue
				}
				for _, r := range s.Requires {
					if r == key || strings.HasPrefix(r, key+".") {
						n++
						break
					}
				}
			}
		}
	}
	return n
}

func (in *Input) order() map[string]int {
	out := map[string]int{}
	for i, v := range in.Volumes.Volumes {
		out[v.ID] = i
	}
	return out
}

// released is true when each section of the plan is reviewed or stale, and at least one is reviewed.
func released(p *plan.Plan) bool {
	reviewed := false
	for _, c := range p.Chapters {
		for _, s := range c.Sections {
			switch s.Status {
			case "reviewed":
				reviewed = true
			case "stale":
			default:
				return false
			}
		}
	}
	return reviewed
}

// Shape is the name shape of a declaration for the family role: the last component of the name, with each digit as #.
func Shape(name string) string {
	last := name
	if i := strings.LastIndex(name, "."); i >= 0 && i+1 < len(name) {
		last = name[i+1:]
	}
	var b strings.Builder
	for _, r := range last {
		if r >= '0' && r <= '9' {
			b.WriteByte('#')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// candidates gives the items of a role before the rank. With all, it drops the status filters, so that a person can give any item with -item.
func (in *Input) candidates(role *Role, all bool) []Item {
	ord := in.order()
	var out []Item
	add := func(it Item) {
		it.Role, it.Kind = role.Name, role.Target
		it.order = ord[it.Volume]
		out = append(out, it)
	}
	switch role.Target {
	case TargetVolume:
		for _, v := range in.Volumes.Volumes {
			if p := in.Plans[v.ID]; p != nil && !all {
				continue
			}
			add(Item{Target: v.ID, Title: v.Title, Volume: v.ID, Why: []string{"the volume has no section plan"}})
		}
	case TargetModule:
		e1 := in.e1()
		for vol, mods := range e1 {
			have := in.fresh(vol)
			for mod, ds := range mods {
				var names []string
				for _, d := range ds {
					if !have[d.Name] {
						names = append(names, d.Name)
					}
				}
				if len(names) == 0 && !all {
					continue
				}
				sort.Strings(names)
				add(Item{Target: mod, Title: fmt.Sprintf("%d of %d E1 declarations have no fresh record", len(names), len(ds)), Volume: vol, Module: mod, Names: names, Size: len(names)})
			}
		}
	case TargetGroup:
		groups := map[string][]string{}
		vols := map[string]map[string]int{}
		for vol, mods := range in.e1() {
			for _, ds := range mods {
				for _, d := range ds {
					if _, ok := in.Families[d.Name]; ok {
						continue
					}
					s := Shape(d.Name)
					groups[s] = append(groups[s], d.Name)
					if vols[s] == nil {
						vols[s] = map[string]int{}
					}
					vols[s][vol]++
				}
			}
		}
		for s, names := range groups {
			if len(names) < MinGroup {
				continue
			}
			sort.Strings(names)
			vol, most := "", 0
			for v, n := range vols[s] {
				if n > most || n == most && v < vol {
					vol, most = v, n
				}
			}
			add(Item{Target: s, Title: fmt.Sprintf("%d E1 declarations with no family end in %s", len(names), s), Volume: vol, Names: names, Size: len(names)})
		}
	case TargetSection:
		want := map[string]string{"writer": "planned", "exampler": "drafted", "exerciser": "drafted", "checker": "drafted", "refresher": "stale"}[role.Name]
		for _, p := range in.Plans {
			if role.Name == "writer" && p.Status != "accepted" && !all {
				continue
			}
			for ci := range p.Chapters {
				c := &p.Chapters[ci]
				for si := range c.Sections {
					s := &c.Sections[si]
					if s.Status != want && !all {
						continue
					}
					n := len(s.E3) + len(s.E2)
					add(Item{Target: s.ID, Title: s.Title, Volume: p.Volume, Plan: p, Chap: c, Sect: s, Size: n, Why: []string{"the section is " + s.Status}})
				}
			}
		}
	}
	return out
}

// Queue gives the ranked queue of a role, the highest rank first (spec 11.5): rank = weight of the volume × (1 + readers that wait on it) × level factor / (1 + failed runs) × age factor. Ties go in the order of the volumes, then by size, then by target.
func (in *Input) Queue(role *Role) []Item {
	return in.rank(role, in.candidates(role, false))
}

// Find gives the item of a role with the target, also when the status filters of the queue leave it out.
func (in *Input) Find(role *Role, target string) (Item, bool) {
	if role.Target == TargetBump {
		it := Item{Role: role.Name, Kind: TargetBump, Target: target, Title: "the failing bump at " + target, Why: []string{"the bump was given with -item"}}
		return in.rank(role, []Item{it})[0], true
	}
	for _, it := range in.rank(role, in.candidates(role, true)) {
		if it.Target == target {
			return it, true
		}
	}
	return Item{}, false
}

func (in *Input) rank(role *Role, items []Item) []Item {
	for i := range items {
		it := &items[i]
		it.Weight = blueprint.Weight(in.Milestones, in.Current, it.Volume)
		it.Why = append(it.Why, fmt.Sprintf("weight %.1f: %s", it.Weight, weightWhy(it.Weight, in.Current)))
		switch {
		case it.Chap != nil:
			it.Readers = in.readers(it.Plan.Volume + "." + it.Chap.ID)
		case it.Kind == TargetVolume:
			it.Readers = in.readers(it.Volume)
		}
		if it.Readers > 0 {
			it.Why = append(it.Why, fmt.Sprintf("%d planned sections wait on it", it.Readers))
		}
		it.Level = 1
		if role.Name == "refresher" && it.Plan != nil && released(it.Plan) {
			it.Level = 2
			it.Why = append(it.Why, "level factor 2: a stale section of a released volume")
		}
		it.Failed = len(FailedRuns(in.Runs, role.Name, it.Target))
		if it.Failed > 0 {
			it.Why = append(it.Why, fmt.Sprintf("%d failed runs", it.Failed))
		}
		it.Age, it.Why = age(in.Runs, role.Name, it.Target, in.Date, it.Why)
		it.Rank = it.Weight * float64(1+it.Readers) * it.Level / float64(1+it.Failed) * it.Age
		it.Rank = math.Round(it.Rank*1000) / 1000
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch {
		case a.Rank != b.Rank:
			return a.Rank > b.Rank
		case a.order != b.order:
			return a.order < b.order
		case a.Kind == TargetModule:
			// The informalizer goes by module, in the order of the volumes (spec 11.5).
			return a.Target < b.Target
		case a.Size != b.Size:
			return a.Size > b.Size
		}
		return a.Target < b.Target
	})
	return items
}

func weightWhy(w float64, cur string) string {
	switch w {
	case 1:
		return "a volume of the current milestone " + cur
	case 0.5:
		return "a volume of the milestone after " + cur
	}
	return "a volume of a later milestone"
}

// age gives the age factor: 1 plus the days since the last run of the role on the target, divided by 28, at most 2. A target with no run has the factor 2, so a target that waits long rises in the queue. This is the age factor of mmp-agent.
func age(runs []*Run, role, target string, now time.Time, why []string) (float64, []string) {
	last := ""
	for _, r := range runs {
		if r.Role == role && r.Target == target && r.Date > last {
			last = r.Date
		}
	}
	if last == "" {
		return 2, append(why, "age factor 2.00: no run yet")
	}
	t, err := time.Parse(time.DateOnly, last)
	if err != nil {
		return 2, append(why, "age factor 2.00: the date of the last run is not a date")
	}
	a := min(2, 1+max(0, now.Sub(t).Hours()/24)/28)
	return a, append(why, fmt.Sprintf("age factor %.2f: the last run was on %s", a, last))
}
