// Package site builds mathlib.tamnd.com from the index of Mathlib, the volume map and the records (spec 13).
//
// The site has one page for each module, with an anchor for each declaration. A page for each declaration would give about 340k files, which is more than a Worker with static assets can hold. The address /decl/<name>/ is not a file: the 404 page reads the shard of the name in decl/shard/ and sends the reader to the module page.
package site

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/coverage"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/index"
	"github.com/yuin/goldmark"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets
var assetFS embed.FS

// Shards is the number of lookup shards in decl/shard/.
const Shards = 256

const (
	docsBase   = "https://leanprover-community.github.io/mathlib4_docs/"
	sourceBase = "https://github.com/leanprover-community/mathlib4/blob/"
	treeBase   = "https://github.com/leanprover-community/mathlib4/tree/"
)

// Input is the data of the site.
type Input struct {
	Index  *index.Index
	Map    *blueprint.VolumeMap
	Depths map[string]depth.Result
	Drift  []drift.Record
}

// Config is the place and the stamp of a build.
type Config struct {
	Out     string // the output directory
	Base    string // the path of the site root, "/" by default
	URL     string // the canonical origin, for example https://mathlib.tamnd.com
	Version string // the version of mlib
	Built   string // the build date
}

// Stats counts what a build wrote.
type Stats struct {
	Files   int
	Modules int
	Decls   int
}

// Shard gives the lookup shard of a declaration name: the low byte of the FNV-1a hash of the name, as two hex digits. lookup.js computes the same value.
func Shard(name string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return fmt.Sprintf("%02x", h.Sum32()&(Shards-1))
}

type menuItem struct{ Path, Section, Text, Question string }

type siteInfo struct {
	Name      string
	Mathlib   string
	Toolchain string
	IndexDate string
	Version   string
	Built     string
	Menu      []menuItem
}

type page struct {
	Site        *siteInfo
	Path        string // the path from the root, with no leading slash, for example "module/Mathlib.Logic.Basic/"
	Section     string
	Title       string
	Description string
	NoIndex     bool
	Lookup      bool
	Content     template.HTML
	Data        any
}

type volRow struct {
	ID, Title, Part, Level string
	Prefixes               []string
	Modules, Decls         int
	E3, E2, E1, E0         int
	Rows                   []modRow
}

type modRow struct {
	Module, Volume string
	Decls, E3, E2  int
}

type declView struct {
	Name, Kind, Module, Type, Doc, Depth, Reason string
	Line                                         int
	Attrs                                        []string
}

type aliasView struct{ Name, Target, Since string }

type moduleView struct {
	Module  string
	Volume  *volRow
	Docs    []string
	Decls   []declView
	Aliases []aliasView
}

type driftView struct {
	drift.Record
	Breaking int
}

type builder struct {
	in    Input
	cfg   Config
	site  *siteInfo
	tmpl  *template.Template
	md    goldmark.Markdown
	mu    sync.Mutex
	files int
	paths []string // the pages for the sitemap
}

// Build writes the site to cfg.Out.
func Build(in Input, cfg Config) (Stats, error) {
	if cfg.Base == "" {
		cfg.Base = "/"
	}
	if !strings.HasSuffix(cfg.Base, "/") {
		cfg.Base += "/"
	}
	cfg.URL = strings.TrimSuffix(cfg.URL, "/")
	h := in.Index.Header
	b := &builder{in: in, cfg: cfg, md: goldmark.New()}
	b.site = &siteInfo{
		Name: "The Mathlib Handbook", Mathlib: h.Mathlib, Toolchain: h.Toolchain, IndexDate: h.Date,
		Version: cfg.Version, Built: cfg.Built,
		Menu: []menuItem{
			{"area/", "area", "Areas", "The 35 volumes and their modules"},
			{"module/", "module", "Modules", "Every module of Mathlib"},
			{"coverage/", "coverage", "Coverage", "The coverage numbers of each volume"},
			{"drift/", "drift", "Drift", "What changed in Mathlib at each sync"},
		},
	}
	if err := b.parse(); err != nil {
		return Stats{}, err
	}
	if err := os.RemoveAll(cfg.Out); err != nil {
		return Stats{}, err
	}
	if err := b.copyAssets(); err != nil {
		return Stats{}, err
	}

	// Group the declarations by module, in the order of the source.
	vols, byVol := b.volumes()
	mods := map[string]*moduleView{}
	for _, m := range in.Index.Modules {
		mods[m.Module] = &moduleView{Module: m.Module, Docs: m.Docs}
	}
	rec, _ := coverage.Compute(in.Index, in.Map, in.Depths, h.Date)
	decls := 0
	for i := range in.Index.Decls {
		d := &in.Index.Decls[i]
		mv, ok := mods[d.Module]
		if !ok {
			mv = &moduleView{Module: d.Module}
			mods[d.Module] = mv
		}
		if d.Deprecated != nil {
			mv.Aliases = append(mv.Aliases, aliasView{d.Name, d.Deprecated.Target, d.Deprecated.Since})
			continue
		}
		r := in.Depths[d.Name]
		mv.Decls = append(mv.Decls, declView{
			Name: d.Name, Kind: d.Kind, Module: d.Module, Type: d.Type, Doc: d.Doc, Depth: r.Depth, Reason: r.Reason,
			Line: d.Line, Attrs: attrsOf(d),
		})
		decls++
	}
	names := make([]string, 0, len(mods))
	for n, mv := range mods {
		names = append(names, n)
		sort.SliceStable(mv.Decls, func(i, j int) bool { return mv.Decls[i].Line < mv.Decls[j].Line })
		sort.Slice(mv.Aliases, func(i, j int) bool { return mv.Aliases[i].Name < mv.Aliases[j].Name })
		vol, _, _ := in.Map.Home(index.ModulePath(n))
		mv.Volume = byVol[vol]
		if mv.Volume == nil {
			mv.Volume = &volRow{ID: "none", Title: "No home volume"}
		}
	}
	sort.Strings(names)

	// The rows of the area pages and of the module list.
	var all []modRow
	for _, n := range names {
		mv := mods[n]
		row := modRow{Module: n, Volume: mv.Volume.ID, Decls: len(mv.Decls)}
		for _, d := range mv.Decls {
			switch d.Depth {
			case depth.E3:
				row.E3++
			case depth.E2:
				row.E2++
			}
		}
		all = append(all, row)
		if v := byVol[mv.Volume.ID]; v != nil {
			v.Rows = append(v.Rows, row)
		}
	}

	// The module pages, in parallel.
	errs := make(chan error, 1)
	work := make(chan *moduleView)
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for mv := range work {
				err := b.page("module", page{
					Path: "module/" + mv.Module + "/", Section: "module", Title: mv.Module,
					Description: fmt.Sprintf("The %d declarations of the Mathlib module %s, with their types, docstrings and depths.", len(mv.Decls), mv.Module),
					Data:        mv,
				})
				if err != nil {
					select {
					case errs <- err:
					default:
					}
				}
			}
		}()
	}
	for _, n := range names {
		work <- mods[n]
	}
	close(work)
	wg.Wait()
	select {
	case err := <-errs:
		return Stats{}, err
	default:
	}

	// The other pages.
	type part struct {
		Name    string
		Volumes []*volRow
	}
	var parts []part
	for _, v := range vols {
		if len(parts) == 0 || parts[len(parts)-1].Name != v.Part {
			parts = append(parts, part{Name: v.Part})
		}
		parts[len(parts)-1].Volumes = append(parts[len(parts)-1].Volumes, v)
	}
	home := struct {
		Decls, Modules, Aliases, E3, E2, E1, E0 int
		Volumes                                 []*volRow
	}{decls, len(names), rec.Aliases, rec.Total.Depth[depth.E3], rec.Total.Depth[depth.E2], rec.Total.Depth[depth.E1], rec.Total.Depth[depth.E0], vols}
	pages := []struct {
		name string
		p    page
	}{
		{"home", page{Path: "", Description: "A textbook of all the mathematics in Mathlib, true to the latest Mathlib. The map of the library: every module, every declaration and its depth.", Data: home}},
		{"areas", page{Path: "area/", Section: "area", Title: "The areas", Description: "The 35 volumes of the Mathlib Handbook and the Mathlib modules of each volume.", Data: struct{ Parts []part }{parts}}},
		{"modules", page{Path: "module/", Section: "module", Title: "The modules", Description: "Every module of Mathlib, with its home volume and its number of declarations.", Data: all}},
		{"decl", page{Path: "decl/", Title: "Go to a declaration", Description: "Open the module page of a Mathlib declaration by its full name.", Lookup: true}},
		{"coverage", page{Path: "coverage/", Section: "coverage", Title: "Coverage", Description: "The coverage numbers of the Mathlib Handbook for each volume.", Data: rec}},
	}
	for _, v := range vols {
		pages = append(pages, struct {
			name string
			p    page
		}{"area", page{Path: "area/" + v.ID + "/", Section: "area", Title: v.ID + ". " + v.Title, Description: fmt.Sprintf("Volume %s of the Mathlib Handbook: %s. %d Mathlib modules and %d declarations.", v.ID, v.Title, v.Modules, v.Decls), Data: v}})
	}
	var dv []driftView
	for _, r := range in.Drift {
		v := driftView{Record: r}
		for _, it := range r.Items {
			if it.Severity == "breaking" {
				v.Breaking++
			}
		}
		dv = append(dv, v)
		pages = append(pages, struct {
			name string
			p    page
		}{"driftday", page{Path: "drift/" + r.Date + "/", Section: "drift", Title: "Drift on " + r.Date, Description: "The changes of Mathlib from " + short(r.From) + " to " + short(r.To) + ".", Data: v}})
	}
	sort.Slice(dv, func(i, j int) bool { return dv[i].Date > dv[j].Date })
	pages = append(pages, struct {
		name string
		p    page
	}{"drift", page{Path: "drift/", Section: "drift", Title: "Drift", Description: "The drift records of the Mathlib Handbook: what changed in Mathlib at each sync.", Data: dv}})
	for _, p := range pages {
		if err := b.page(p.name, p.p); err != nil {
			return Stats{}, err
		}
	}
	if err := b.notFound(); err != nil {
		return Stats{}, err
	}
	if err := b.shards(); err != nil {
		return Stats{}, err
	}
	if err := b.machineFiles(rec, vols); err != nil {
		return Stats{}, err
	}
	return Stats{Files: b.files, Modules: len(names), Decls: decls}, nil
}

func attrsOf(d *index.Decl) []string {
	var out []string
	for _, a := range d.Attrs {
		if a != "class" && a != "instance" {
			out = append(out, a)
		}
	}
	return out
}

func (b *builder) volumes() ([]*volRow, map[string]*volRow) {
	byVol := map[string]*volRow{}
	var vols []*volRow
	for _, v := range b.in.Map.Volumes {
		r := &volRow{ID: v.ID, Title: v.Title, Part: v.Part, Level: v.Level, Prefixes: v.Prefixes}
		vols = append(vols, r)
		byVol[v.ID] = r
	}
	seen := map[string]bool{}
	for i := range b.in.Index.Decls {
		d := &b.in.Index.Decls[i]
		vol, _, ok := b.in.Map.Home(index.ModulePath(d.Module))
		if !ok {
			continue
		}
		r := byVol[vol]
		if !seen[d.Module] {
			seen[d.Module] = true
			r.Modules++
		}
		if d.Deprecated != nil {
			continue
		}
		r.Decls++
		switch b.in.Depths[d.Name].Depth {
		case depth.E3:
			r.E3++
		case depth.E2:
			r.E2++
		case depth.E1:
			r.E1++
		case depth.E0:
			r.E0++
		}
	}
	for _, m := range b.in.Index.Modules {
		if vol, _, ok := b.in.Map.Home(index.ModulePath(m.Module)); ok && !seen[m.Module] {
			seen[m.Module] = true
			byVol[vol].Modules++
		}
	}
	return vols, byVol
}

func (b *builder) parse() error {
	funcs := template.FuncMap{
		"url":       b.url,
		"canonical": func(p string) string { return b.cfg.URL + b.url(p) },
		"short":     short,
		"num":       num,
		"pct":       func(f float64) string { return strconv.FormatFloat(100*f, 'f', 0, 64) + "%" },
		"md":        b.markdown,
		"docsURL": func(mod, name string) string {
			u := docsBase + index.ModulePath(mod) + ".html"
			if name != "" {
				u += "#" + name
			}
			return u
		},
		"srcURL": func(mod string, line int) string {
			u := sourceBase + b.in.Index.Header.Mathlib + "/" + index.ModulePath(mod) + ".lean"
			if line > 0 {
				u += "#L" + strconv.Itoa(line)
			}
			return u
		},
		"mathlibTree": func() string { return treeBase + b.in.Index.Header.Mathlib },
	}
	t, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	b.tmpl = t
	return nil
}

func (b *builder) url(p string) string { return b.cfg.Base + strings.TrimPrefix(p, "/") }

func (b *builder) markdown(s string) template.HTML {
	var buf bytes.Buffer
	// goldmark escapes raw HTML by default, so a docstring cannot add markup to the page.
	if err := b.md.Convert([]byte(s), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(s)) //nolint:gosec // the text is escaped
	}
	return template.HTML(buf.String()) //nolint:gosec // goldmark escapes raw HTML in its default mode
}

// page renders a page into the layout and writes it to <path>/index.html.
func (b *builder) page(name string, p page) error {
	p.Site = b.site
	var body bytes.Buffer
	if err := b.tmpl.ExecuteTemplate(&body, name, p); err != nil {
		return fmt.Errorf("%s: %w", p.Path, err)
	}
	p.Content = template.HTML(body.String()) //nolint:gosec // the body is the output of html/template
	var out bytes.Buffer
	if err := b.tmpl.ExecuteTemplate(&out, "layout", p); err != nil {
		return fmt.Errorf("%s: %w", p.Path, err)
	}
	if err := b.write(path.Join(p.Path, "index.html"), out.Bytes()); err != nil {
		return err
	}
	if !p.NoIndex {
		b.mu.Lock()
		b.paths = append(b.paths, p.Path)
		b.mu.Unlock()
	}
	return nil
}

func (b *builder) notFound() error {
	p := page{Site: b.site, Path: "404.html", Title: "Page not found", Description: "This page does not exist.", NoIndex: true, Lookup: true}
	var body bytes.Buffer
	if err := b.tmpl.ExecuteTemplate(&body, "notfound", p); err != nil {
		return err
	}
	p.Content = template.HTML(body.String()) //nolint:gosec // the body is the output of html/template
	var out bytes.Buffer
	if err := b.tmpl.ExecuteTemplate(&out, "layout", p); err != nil {
		return err
	}
	return b.write("404.html", out.Bytes())
}

// shards writes decl/shard/<xx>.json. Each shard maps the names of its declarations and deprecated aliases to their modules.
func (b *builder) shards() error {
	shards := make([]map[string]string, Shards)
	for i := range shards {
		shards[i] = map[string]string{}
	}
	for i := range b.in.Index.Decls {
		d := &b.in.Index.Decls[i]
		n, _ := strconv.ParseUint(Shard(d.Name), 16, 8)
		shards[n][d.Name] = d.Module
	}
	for i, s := range shards {
		data, err := json.Marshal(s)
		if err != nil {
			return err
		}
		if err := b.write(fmt.Sprintf("decl/shard/%02x.json", i), data); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) machineFiles(rec coverage.Record, vols []*volRow) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := b.write("api/coverage.json", data); err != nil {
		return err
	}
	type vol struct {
		ID       string   `json:"id"`
		Title    string   `json:"title"`
		Part     string   `json:"part"`
		Level    string   `json:"level"`
		Prefixes []string `json:"prefixes"`
		Modules  []string `json:"modules"`
	}
	var vs []vol
	for _, v := range vols {
		mods := make([]string, 0, len(v.Rows))
		for _, r := range v.Rows {
			mods = append(mods, r.Module)
		}
		vs = append(vs, vol{v.ID, v.Title, v.Part, v.Level, v.Prefixes, mods})
	}
	if data, err = json.MarshalIndent(vs, "", "  "); err != nil {
		return err
	}
	if err := b.write("api/volumes.json", data); err != nil {
		return err
	}

	sort.Strings(b.paths)
	var sm bytes.Buffer
	sm.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, p := range b.paths {
		u := b.cfg.URL + b.url((&url.URL{Path: p}).EscapedPath())
		fmt.Fprintf(&sm, "<url><loc>%s</loc></url>\n", template.HTMLEscapeString(u))
	}
	sm.WriteString("</urlset>\n")
	if err := b.write("sitemap.xml", sm.Bytes()); err != nil {
		return err
	}
	robots := "User-agent: *\nAllow: /\nSitemap: " + b.cfg.URL + b.url("sitemap.xml") + "\n"
	if err := b.write("robots.txt", []byte(robots)); err != nil {
		return err
	}
	h := b.in.Index.Header
	var llms bytes.Buffer
	fmt.Fprintf(&llms, "# The Mathlib Handbook\n\n> A textbook of all the mathematics in Mathlib, true to the latest Mathlib. The volumes are not written yet. This site shows the map of Mathlib %s (Lean %s) that the volumes follow.\n\n", h.Mathlib, h.Toolchain)
	fmt.Fprintf(&llms, "## Pages\n\n- [The areas](%s): the 35 volumes and their modules\n- [The modules](%s): every module of Mathlib\n- [Coverage](%s): the coverage numbers\n- [Drift](%s): the changes of Mathlib at each sync\n\n", b.cfg.URL+b.url("area/"), b.cfg.URL+b.url("module/"), b.cfg.URL+b.url("coverage/"), b.cfg.URL+b.url("drift/"))
	fmt.Fprintf(&llms, "## Data\n\n- [api/coverage.json](%s)\n- [api/volumes.json](%s)\n- [The index assets](https://github.com/tamnd/mathlib/releases/tag/index): one JSON line for each declaration of Mathlib\n\nThe page of a module is module/<module name>/, and each declaration has the anchor #<full name>. The address decl/<full name>/ opens the module page at the declaration.\n", b.cfg.URL+b.url("api/coverage.json"), b.cfg.URL+b.url("api/volumes.json"))
	return b.write("llms.txt", llms.Bytes())
}

func (b *builder) copyAssets() error {
	return fs.WalkDir(assetFS, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := assetFS.ReadFile(p)
		if err != nil {
			return err
		}
		return b.write(p, data)
	})
}

func (b *builder) write(rel string, data []byte) error {
	p := filepath.Join(b.cfg.Out, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return err
	}
	b.mu.Lock()
	b.files++
	b.mu.Unlock()
	return nil
}

func short(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

func num(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		return "-" + num(-n)
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
