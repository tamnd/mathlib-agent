package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/mathlib-agent/internal/blueprint"
	"github.com/tamnd/mathlib-agent/internal/drift"
	"github.com/tamnd/mathlib-agent/internal/site"
)

func runSite(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: mlib site build|check [flags]")
		return errUsage
	}
	switch args[0] {
	case "build":
		return runSiteBuild(args[1:], stdout, stderr)
	case "check":
		return runSiteCheck(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "mlib site: unknown command %q\n", args[0])
	return errUsage
}

func runSiteBuild(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("site build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("index", "index", "the directory of the index")
	lists := fs.String("lists", "", "a Mathlib checkout with the curated lists in docs/; the default is the index directory")
	mapDir := fs.String("map", ".", "the checkout of tamnd/mathlib")
	out := fs.String("out", "public", "the output directory")
	base := fs.String("base", "/", "the path of the site root")
	origin := fs.String("url", "https://mathlib.tamnd.com", "the canonical origin of the site")
	if err := fs.Parse(args); err != nil {
		return err
	}
	vm, err := blueprint.LoadVolumes(filepath.Join(*mapDir, "blueprint", "volumes.yaml"))
	if err != nil {
		return err
	}
	ix, depths, _, err := loadDepths(*dir, *lists)
	if err != nil {
		return err
	}
	records, err := loadDrift(filepath.Join(*mapDir, "records", "drift"))
	if err != nil {
		return err
	}
	start := time.Now()
	st, err := site.Build(site.Input{Index: ix, Map: vm, Depths: depths, Drift: records}, site.Config{
		Out: *out, Base: *base, URL: *origin, Version: Version, Built: time.Now().UTC().Format("2006-01-02"),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %d files to %s: %d module pages, %d declarations, in %s\n", st.Files, *out, st.Modules, st.Decls, time.Since(start).Round(time.Millisecond))
	return nil
}

func runSiteCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("site check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "public", "the directory of the built site")
	base := fs.String("base", "/", "the path of the site root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	r, err := site.Check(*out, *base)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d files, %d pages, %d internal links\n", r.Files, r.Pages, r.Links)
	const most = 50
	for i, p := range r.Problems {
		if i == most {
			fmt.Fprintf(stdout, "and %d more problems\n", len(r.Problems)-most)
			break
		}
		fmt.Fprintln(stdout, p)
	}
	if len(r.Problems) > 0 {
		return fmt.Errorf("%d problems: %w", len(r.Problems), errFailed)
	}
	return nil
}

// loadDrift reads the drift records in dir, oldest first. A missing directory gives no records.
func loadDrift(dir string) ([]drift.Record, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []drift.Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(filepath.Join(dir, e.Name())))
		if err != nil {
			return nil, err
		}
		var r drift.Record
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}
