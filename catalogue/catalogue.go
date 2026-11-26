// Package catalogue makes the rows of the Catalogue volumes of spec 06.4. There is one Catalogue volume for each main volume, C05 for V05. It lists every declaration whose home is in the main volume, except the deprecated aliases, as the coverage of spec 04.5 does, in module order, with the kind, the attributes, the depth, a sentence and the home section.
//
// The sentence comes from the first source that has one:
//   - E1: an informal record whose type hash is the hash of the index, with no failed check (spec 09.3 and 14.3);
//   - family: the E0 family of the declaration (spec 09.4);
//   - doc: the first sentence of the docstring of Mathlib, until the informalizer writes a record;
//   - none: no sentence yet.
package catalogue

import (
	"sort"
	"strings"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/family"
	"github.com/tamnd/mathlib-agent/index"
	"github.com/tamnd/mathlib-agent/informal"
	"github.com/tamnd/mathlib-agent/plan"
)

// The sources of a sentence.
const (
	FromE1     = "E1"
	FromFamily = "family"
	FromDoc    = "doc"
	FromNone   = "none"
)

// Row is one declaration of a Catalogue volume.
type Row struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Module  string   `json:"module"`
	Attrs   []string `json:"attrs,omitempty"`
	Depth   string   `json:"depth,omitempty"`
	Source  string   `json:"source"`
	Text    string   `json:"text,omitempty"`
	Family  string   `json:"family,omitempty"`
	Section string   `json:"section,omitempty"`
}

// Header is the first line of a Catalogue file.
type Header struct {
	Catalogue string         `json:"catalogue"`
	Volume    string         `json:"volume"`
	Title     string         `json:"title"`
	Mathlib   string         `json:"mathlib"`
	Date      string         `json:"date"`
	Count     int            `json:"count"`
	Sources   map[string]int `json:"sources"`
}

// Input is what Build reads. Plans and Records are keyed by the volume id. A volume with no plan gets no home sections, and a volume with no records gets no E1 sentences.
type Input struct {
	Index    *index.Index
	Volumes  *blueprint.VolumeMap
	Depths   map[string]depth.Result
	Families []*family.Family
	Match    family.Result
	Plans    map[string]*plan.Plan
	Records  map[string][]informal.Record
	Date     string
}

// ID is the id of the Catalogue volume of a main volume: C05 for V05.
func ID(vol string) string { return "C" + strings.TrimPrefix(vol, "V") }

// Build gives the header and the rows of the Catalogue volume of vol.
func Build(in Input, vol string) (Header, []Row) {
	text := map[string]string{}
	for _, f := range in.Families {
		text[f.ID] = f.Text
	}
	e1 := map[string]informal.Record{}
	for _, r := range in.Records[vol] {
		e1[r.Name] = r
	}
	byName, byModule := sections(in.Plans[vol])
	var decls []*index.Decl
	for i := range in.Index.Decls {
		d := &in.Index.Decls[i]
		if d.Deprecated != nil {
			continue
		}
		if v, _, ok := in.Volumes.Home(d.Path()); ok && v == vol {
			decls = append(decls, d)
		}
	}
	sort.SliceStable(decls, func(i, j int) bool {
		if decls[i].Module != decls[j].Module {
			return decls[i].Module < decls[j].Module
		}
		return decls[i].Line < decls[j].Line
	})
	h := Header{Catalogue: ID(vol), Volume: vol, Mathlib: in.Index.Header.Mathlib, Date: in.Date, Count: len(decls), Sources: map[string]int{}}
	for _, v := range in.Volumes.Volumes {
		if v.ID == vol {
			h.Title = v.Title
		}
	}
	rows := make([]Row, 0, len(decls))
	for _, d := range decls {
		r := Row{Name: d.Name, Kind: d.Kind, Module: d.Module, Attrs: d.Attrs, Depth: in.Depths[d.Name].Depth, Source: FromNone}
		if s, ok := byName[d.Name]; ok {
			r.Section = s
		} else {
			r.Section = byModule[d.Module]
		}
		fam := in.Match.Member[d.Name]
		r.Family = fam
		if rec, ok := e1[d.Name]; ok && usable(rec, d) {
			r.Source, r.Text = FromE1, rec.Text
		} else if fam != "" {
			r.Source, r.Text = FromFamily, text[fam]
		} else if s := FirstSentence(d.Doc); s != "" {
			r.Source, r.Text = FromDoc, s
		}
		h.Sources[r.Source]++
		rows = append(rows, r)
	}
	return h, rows
}

// usable tells whether an informal record can go to print: it is for the type of the index, and no check failed.
func usable(r informal.Record, d *index.Decl) bool {
	if r.Text == "" || r.TypeHash != d.TypeHash {
		return false
	}
	c := r.Checks
	return c.Roundtrip != informal.Fail && c.Critic != informal.Fail && c.Human != informal.Fail
}

// sections gives the home section of each E3 and E2 name of a plan, and of each module of a plan. The first section that names a module is its home.
func sections(p *plan.Plan) (byName, byModule map[string]string) {
	byName, byModule = map[string]string{}, map[string]string{}
	if p == nil {
		return byName, byModule
	}
	for _, c := range p.Chapters {
		for _, s := range c.Sections {
			for _, n := range append(append([]string{}, s.E3...), s.E2...) {
				if _, ok := byName[n]; !ok {
					byName[n] = s.ID
				}
			}
			for _, m := range s.Modules {
				if _, ok := byModule[m]; !ok {
					byModule[m] = s.ID
				}
			}
		}
	}
	return byName, byModule
}

// FirstSentence gives the first sentence of a docstring, on one line. A docstring that starts with a code block or a heading gives the empty string.
func FirstSentence(doc string) string {
	doc = strings.TrimSpace(doc)
	if doc == "" || strings.HasPrefix(doc, "```") || strings.HasPrefix(doc, "#") {
		return ""
	}
	para, _, _ := strings.Cut(doc, "\n\n")
	para = strings.Join(strings.Fields(para), " ")
	inCode := false
	for i, c := range para {
		switch {
		case c == '`':
			inCode = !inCode
		case !inCode && c == '.' && (i+1 == len(para) || para[i+1] == ' '):
			return para[:i+1]
		}
	}
	if len(para) > 400 {
		return ""
	}
	return para
}
