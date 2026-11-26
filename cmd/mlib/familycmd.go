package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/tamnd/mathlib-agent/internal/family"
	"github.com/tamnd/mathlib-agent/internal/index"
)

// familiesPath is the place of the family records in a checkout of tamnd/mathlib.
func familiesPath(mapDir string) string {
	return filepath.Join(mapDir, "blueprint", "families.yaml")
}

func runFamily(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: give a subcommand: match, list or show", errUsage)
	}
	sub, args := args[0], args[1:]
	fs := flag.NewFlagSet("family "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	mapDir := fs.String("map", ".", "the checkout of tamnd/mathlib")
	dir := fs.String("index", "index", "the directory of the index")
	var out, date *string
	var sample *int
	if sub == "match" {
		out = fs.String("out", "", "write records/families.json to this file")
		date = fs.String("date", time.Now().UTC().Format(time.DateOnly), "the date of the record")
		sample = fs.Int("sample", 5, "the number of sample members of each family in the record")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	fams, err := family.Load(familiesPath(*mapDir))
	if err != nil {
		return err
	}
	switch sub {
	case "match", "list", "show":
	default:
		return fmt.Errorf("%w: unknown subcommand %q", errUsage, sub)
	}
	ix, err := index.Load(*dir)
	if err != nil {
		return err
	}
	r := family.Match(ix, fams)
	switch sub {
	case "match":
		rec := family.Summarize(ix, fams, r, *date, *sample)
		fmt.Fprintf(stdout, "%d families, %d of %d declarations are members (%.1f%%)\n", len(fams), rec.Members, len(ix.Decls), 100*float64(rec.Members)/float64(max(len(ix.Decls), 1)))
		if *out != "" {
			if err := writeJSON(*out, rec); err != nil {
				return err
			}
			fmt.Fprintln(stdout, "wrote", *out)
		}
		return nil
	case "list":
		tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "family\tmembers\ttext")
		for _, f := range fams {
			fmt.Fprintf(tw, "%s\t%d\t%s\n", f.ID, r.Count[f.ID], f.Text)
		}
		return tw.Flush()
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("%w: give one family, for example mlib family show F-map-op", errUsage)
	}
	id := fs.Arg(0)
	var f *family.Family
	for _, g := range fams {
		if g.ID == id {
			f = g
		}
	}
	if f == nil {
		return fmt.Errorf("blueprint/families.yaml has no family %s", id)
	}
	fmt.Fprintf(stdout, "%s: %s\n%d members\n", f.ID, f.Text, r.Count[id])
	var names []string
	for n, m := range r.Member {
		if m == id {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if src, ok := r.Twin[n]; ok {
			fmt.Fprintf(stdout, "%s\ttwin of %s\n", n, src)
		} else {
			fmt.Fprintln(stdout, n)
		}
	}
	return nil
}
