package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/plan"
	"github.com/tamnd/mathlib-agent/rename"
	"github.com/tamnd/mathlib-agent/stale"
)

func runStale(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("stale", flag.ContinueOnError)
	fl.SetOutput(stderr)
	mapDir := fl.String("map", ".", "the checkout of tamnd/mathlib, with records/drift/ and blueprint/sections/")
	book := fl.String("book", "", "a checkout of tamnd/mathlib-handbook; the section files are content/**/*.md")
	to := fl.String("to", "", "the new commit of Mathlib; empty for the last commit of the drift records")
	date := fl.String("date", time.Now().UTC().Format("2006-01-02"), "the date of the record")
	apply := fl.Bool("apply", false, "write the marks to the sections, the plans and records/stale.json; without it, list them only")
	asJSON := fl.Bool("json", false, "print the record as JSON")
	if err := fl.Parse(args); err != nil {
		return err
	}
	if *book == "" {
		return fmt.Errorf("%w: give -book, the checkout of tamnd/mathlib-handbook", errUsage)
	}
	res, err := staleMarks(*mapDir, *book, *to, *date, *apply)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res.rec)
	}
	writeStale(stdout, res)
	if *apply && len(res.rec.Marks) > 0 {
		fmt.Fprintf(stdout, "wrote %d section files and %d plans; open a pull request to tamnd/mathlib-handbook with the sections\n", res.files, res.plans)
	}
	return nil
}

// staleResult is what staleMarks did.
type staleResult struct {
	rec      *stale.Record
	sections int
	noChain  []string // the sections whose commit has no chain of drift records to the new commit
	files    int
	plans    int
}

// lastCommit is the commit that no drift record starts from: the end of the chain.
func lastCommit(recs []drift.Record) string {
	from := map[string]bool{}
	for _, r := range recs {
		from[r.From] = true
	}
	for i := len(recs) - 1; i >= 0; i-- {
		if !from[recs[i].To] {
			return recs[i].To
		}
	}
	return ""
}

// staleMarks finds the stale sections of the book at the commit to (spec 05.5). With apply it writes the marks.
func staleMarks(mapDir, book, to, date string, apply bool) (*staleResult, error) {
	recs, err := rename.LoadDrift(mapDir)
	if err != nil {
		return nil, err
	}
	if to == "" {
		to = lastCommit(recs)
	}
	if to == "" {
		return nil, errors.New("records/drift/ has no drift record; run mlib sync first")
	}
	res := &staleResult{rec: &stale.Record{Mathlib: to, Date: date, Marks: []stale.Mark{}}}
	root, err := os.OpenRoot(book)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	type write struct{ path, text string }
	var writes []write
	err = fs.WalkDir(root.FS(), "content", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") || stale.ID(path) == "" {
			return err
		}
		b, err := root.ReadFile(path)
		if err != nil {
			return err
		}
		s, ok, err := stale.Parse(path, string(b))
		if err != nil || !ok || s.Status == "planned" {
			return err
		}
		res.sections++
		chain, err := rename.Chain(recs, s.Mathlib, to)
		if err != nil {
			res.noChain = append(res.noChain, s.ID+" at "+short(s.Mathlib))
			return nil //nolint:nilerr // a section with no chain is listed, and the walk goes on
		}
		m, ok := stale.Find(s, chain)
		if !ok {
			return nil
		}
		res.rec.Marks = append(res.rec.Marks, m)
		out, err := stale.Apply(string(b), m)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if out != string(b) {
			writes = append(writes, write{path, out})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(res.rec.Marks, func(i, j int) bool { return res.rec.Marks[i].Section < res.rec.Marks[j].Section })
	if !apply {
		return res, nil
	}
	for _, w := range writes {
		if err := root.WriteFile(w.path, []byte(w.text), 0o644); err != nil { //nolint:gosec // a file of the book is read by the site build, so others can read it
			return nil, err
		}
	}
	res.files = len(writes)
	n, err := markPlans(mapDir, res.rec.Marks)
	if err != nil {
		return nil, err
	}
	res.plans = n
	return res, writeJSON(stale.Path(mapDir), res.rec)
}

// markPlans sets the status stale on the sections of the plans that have a mark, so that the queue of the refresher takes them (spec 11.5).
func markPlans(mapDir string, marks []stale.Mark) (int, error) {
	byVol := map[string]map[string]bool{}
	for _, m := range marks {
		vol, _, _ := strings.Cut(m.Section, ".")
		if byVol[vol] == nil {
			byVol[vol] = map[string]bool{}
		}
		byVol[vol][m.Section] = true
	}
	n := 0
	for vol, ids := range byVol {
		p, err := plan.Load(plan.Path(mapDir, vol))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return n, err
		}
		changed := false
		for ci := range p.Chapters {
			for si := range p.Chapters[ci].Sections {
				s := &p.Chapters[ci].Sections[si]
				if ids[s.ID] && s.Status != "stale" {
					s.Status, changed = "stale", true
				}
			}
		}
		if changed {
			if err := p.Save(plan.Path(mapDir, vol)); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func writeStale(w io.Writer, res *staleResult) {
	fmt.Fprintf(w, "%d sections at Mathlib %s, %d stale\n", res.sections, short(res.rec.Mathlib), len(res.rec.Marks))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, m := range res.rec.Marks {
		kind := "minor"
		if m.Breaking() {
			kind = "breaking"
		}
		fmt.Fprintf(tw, "%s\t%s\tsince %s\twas %s\t%d items\n", m.Section, kind, short(m.Since), m.From, len(m.Items))
		for _, h := range m.Items {
			fmt.Fprintf(tw, "\t%s\t%s\t%s\t%s\n", h.Severity, h.Class, h.Name, h.Why)
		}
	}
	_ = tw.Flush()
	for _, s := range res.noChain {
		fmt.Fprintf(w, "no chain of drift records: %s\n", s)
	}
}
