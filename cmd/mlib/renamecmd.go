package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/tamnd/mathlib-agent/informal"
	"github.com/tamnd/mathlib-agent/rename"
)

func runRename(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("rename", flag.ContinueOnError)
	fl.SetOutput(stderr)
	mapDir := fl.String("map", ".", "the checkout of tamnd/mathlib, with records/drift/ and informal/")
	book := fl.String("book", "", "a checkout of tamnd/mathlib-handbook; the section files are content/**/*.md")
	lean := fl.String("lean", "", "a checkout of tamnd/mathlib-lean; the files are MathlibHandbook/**/*.lean")
	from := fl.String("from", "", "the commit of Mathlib that the text was written for")
	to := fl.String("to", "", "the new commit of Mathlib")
	proposals := fl.Bool("proposals", false, "also take the renames that a type hash match proposes; a person must accept them first")
	apply := fl.Bool("apply", false, "write the changes; without it, list them only")
	asJSON := fl.Bool("json", false, "print the table and the changes as JSON")
	if err := fl.Parse(args); err != nil {
		return err
	}
	if *from == "" || *to == "" {
		return fmt.Errorf("%w: give -from and -to, for example mlib rename -from 4472534b60 -to e54979a532", errUsage)
	}
	recs, err := rename.LoadDrift(*mapDir)
	if err != nil {
		return err
	}
	chain, err := rename.Chain(recs, *from, *to)
	if err != nil {
		return err
	}
	table := rename.Table(chain, *proposals)
	var changes []rename.Change
	// A write keeps the root of its checkout, so that the walk, the read and the write cannot leave the checkout through a symbolic link.
	type write struct {
		root       *os.Root
		path, text string
	}
	var writes []write
	var records []func() error
	walk := func(dir, sub, ext string, fn func(path, src string) (string, []rename.Change)) error {
		if dir == "" {
			return nil
		}
		if _, err := os.Stat(filepath.Join(dir, sub)); errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		root, err := os.OpenRoot(dir)
		if err != nil {
			return err
		}
		return fs.WalkDir(root.FS(), sub, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ext) {
				return err
			}
			b, err := root.ReadFile(path)
			if err != nil {
				return err
			}
			out, ch := fn(path, string(b))
			if len(ch) > 0 {
				changes = append(changes, ch...)
				writes = append(writes, write{root, path, out})
			}
			return nil
		})
	}
	if err := walk(*book, "content", ".md", func(p, s string) (string, []rename.Change) { return rename.Markdown(p, s, table) }); err != nil {
		return err
	}
	if err := walk(*lean, "MathlibHandbook", ".lean", func(p, s string) (string, []rename.Change) { return rename.Lean(p, s, table) }); err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(*mapDir, "informal", "*.jsonl"))
	if err != nil {
		return err
	}
	for _, p := range paths {
		rs, err := informal.Load(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(*mapDir, p)
		rs, ch := rename.Informal(rel, rs, table)
		if len(ch) > 0 {
			changes = append(changes, ch...)
			records = append(records, func() error { return informal.Save(p, rs) })
		}
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Table   []rename.Entry  `json:"table"`
			Changes []rename.Change `json:"changes"`
		}{table, changes}); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(stdout, "%d drift records from %s to %s, %d renames, %d changes\n", len(chain), *from, *to, len(table), len(changes))
		tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		for _, e := range table {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Kind, e.Old, e.New, e.Note)
		}
		if len(changes) > 0 {
			fmt.Fprintln(tw, "\nfile\tline\told\tnew")
		}
		for _, c := range changes {
			fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", c.File, c.Line, c.Old, c.New)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	if !*apply {
		return nil
	}
	for _, w := range writes {
		if err := w.root.WriteFile(w.path, []byte(w.text), 0o644); err != nil { //nolint:gosec // a file of the book is read by the site build, so others can read it
			return err
		}
	}
	for _, f := range records {
		if err := f(); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "wrote %d files and %d record files; open a pull request in each repository with the changes\n", len(writes), len(records))
	return nil
}
