package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/catalogue"
	"github.com/tamnd/mathlib-agent/family"
	"github.com/tamnd/mathlib-agent/informal"
	"github.com/tamnd/mathlib-agent/plan"
)

// runCatalogue writes the rows of the Catalogue volumes (spec 06.4) as one file C<NN>.jsonl.gz for each volume. mlhb catalogue of tamnd/mathlib-handbook sets the files as books.
func runCatalogue(args []string, stdout, stderr io.Writer) error {
	f := flag.NewFlagSet("catalogue", flag.ContinueOnError)
	f.SetOutput(stderr)
	dir := f.String("index", "index", "the directory of the index")
	lists := f.String("lists", "", "a Mathlib checkout with the curated lists in docs/; the default is the index directory")
	mapDir := f.String("map", ".", "the checkout of tamnd/mathlib")
	vols := f.String("volume", "all", "the main volumes, as V05,V13, or all")
	out := f.String("out", "catalogue", "the directory of the files")
	date := f.String("date", time.Now().UTC().Format(time.DateOnly), "the date of the files")
	if err := f.Parse(args); err != nil {
		return err
	}
	vm, err := blueprint.LoadVolumes(filepath.Join(*mapDir, "blueprint", "volumes.yaml"))
	if err != nil {
		return err
	}
	fams, err := family.Load(familiesPath(*mapDir))
	if err != nil {
		return err
	}
	ix, depths, _, err := loadDepths(*dir, *lists)
	if err != nil {
		return err
	}
	var ids []string
	for _, v := range vm.Volumes {
		if *vols == "all" || hasItem(*vols, v.ID) {
			ids = append(ids, v.ID)
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: no volume %s in blueprint/volumes.yaml", errUsage, *vols)
	}
	in := catalogue.Input{Index: ix, Volumes: vm, Depths: depths, Families: fams, Match: family.Match(ix, fams),
		Plans: map[string]*plan.Plan{}, Records: map[string][]informal.Record{}, Date: *date}
	for _, v := range ids {
		if p, err := plan.Load(plan.Path(*mapDir, v)); err == nil {
			in.Plans[v] = p
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		rs, err := informal.Load(informal.Path(*mapDir, v))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		in.Records[v] = rs
	}
	if err := os.MkdirAll(*out, 0o750); err != nil {
		return err
	}
	for _, v := range ids {
		h, rows := catalogue.Build(in, v)
		path := filepath.Join(*out, h.Catalogue+".jsonl.gz")
		if err := writeRows(path, h, rows); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s %s: %d declarations, E1 %d, family %d, doc %d, none %d\n", h.Catalogue, v, h.Count,
			h.Sources[catalogue.FromE1], h.Sources[catalogue.FromFamily], h.Sources[catalogue.FromDoc], h.Sources[catalogue.FromNone])
	}
	return nil
}

func hasItem(list, x string) bool {
	for _, s := range strings.Split(list, ",") {
		if strings.TrimSpace(s) == x {
			return true
		}
	}
	return false
}

func writeRows(path string, h catalogue.Header, rows []catalogue.Row) (err error) {
	file, err := os.Create(filepath.Clean(path)) //nolint:gosec // the path comes from the -out flag of the user
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); err == nil {
			err = cerr
		}
	}()
	zw := gzip.NewWriter(file)
	bw := bufio.NewWriter(zw)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(h); err != nil {
		return err
	}
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return zw.Close()
}
