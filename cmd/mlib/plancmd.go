package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/family"
	"github.com/tamnd/mathlib-agent/plan"
)

func runPlan(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mapDir := fs.String("map", ".", "the checkout of tamnd/mathlib")
	dir := fs.String("index", "index", "the directory of the index")
	lists := fs.String("lists", "", "a Mathlib checkout with the curated lists in docs/; the default is the index directory")
	vols := fs.String("volume", "", "the volume, or a list of volumes with commas, for example V13 or V00,V01")
	pages := fs.Float64("pages", plan.Defaults.SectionPages, "the size of a section in pages that the plan aims at")
	write := fs.Bool("write", false, "write blueprint/sections/<vol>.yaml; without it, print a summary only")
	force := fs.Bool("force", false, "write over a plan that a person has accepted")
	date := fs.String("date", time.Now().UTC().Format(time.DateOnly), "the date of the plan")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *vols == "" {
		return fmt.Errorf("%w: give -volume, for example mlib plan -volume V13", errUsage)
	}
	vm, err := blueprint.LoadVolumes(filepath.Join(*mapDir, "blueprint", "volumes.yaml"))
	if err != nil {
		return err
	}
	ix, depths, _, err := loadDepths(*dir, *lists)
	if err != nil {
		return err
	}
	in := plan.Input{Index: ix, Volumes: vm, Depths: depths}
	if _, err := os.Stat(familiesPath(*mapDir)); err == nil {
		fams, err := family.Load(familiesPath(*mapDir))
		if err != nil {
			return err
		}
		in.Families = family.Match(ix, fams).Member
	}
	o := plan.Defaults
	o.SectionPages, o.Date, o.By = *pages, *date, "mlib plan "+Version
	for _, v := range strings.Split(*vols, ",") {
		p, err := plan.Make(in, strings.TrimSpace(v), o)
		if err != nil {
			return err
		}
		n, e3, e2 := 0, 0, 0
		for _, c := range p.Chapters {
			n += len(c.Sections)
			for _, s := range c.Sections {
				e3 += len(s.E3)
				e2 += len(s.E2)
			}
		}
		fmt.Fprintf(stdout, "%s: %d chapters, %d sections, %d E3, %d E2, about %.0f pages\n", p.Volume, len(p.Chapters), n, e3, e2, p.Pages)
		if !*write {
			continue
		}
		path := plan.Path(*mapDir, p.Volume)
		if old, err := plan.Load(path); err == nil && old.Status == "accepted" && !*force {
			return fmt.Errorf("%s is accepted; give -force to write over it", path)
		}
		if err := p.Save(path); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "wrote", path)
	}
	return nil
}
