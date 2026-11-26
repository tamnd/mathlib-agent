package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/edition"
	"github.com/tamnd/mathlib-agent/family"
	"github.com/tamnd/mathlib-agent/informal"
)

// errFailed is a failed check. The details are already written. main exits with code 1.
var errFailed = errors.New("the check failed")

// mapFlags are the flags that say where the map and the list of modules are.
type mapFlags struct {
	mapDir  string
	mathlib string
	modules string
}

func (f *mapFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.mapDir, "map", ".", "the checkout of tamnd/mathlib")
	fs.StringVar(&f.mathlib, "mathlib", "", "a checkout of Mathlib, to read the modules from its Mathlib/ folder")
	fs.StringVar(&f.modules, "modules", "", "a file with one module path per line, such as Mathlib/Topology/Basic")
}

func (f *mapFlags) volumes() (*blueprint.VolumeMap, error) {
	return blueprint.LoadVolumes(filepath.Join(f.mapDir, "blueprint", "volumes.yaml"))
}

// moduleList reads the modules from -modules or -mathlib.
func (f *mapFlags) moduleList() ([]string, error) {
	switch {
	case f.modules != "" && f.mathlib != "":
		return nil, fmt.Errorf("%w: give -modules or -mathlib, not both", errUsage)
	case f.mathlib != "":
		return blueprint.Modules(f.mathlib)
	case f.modules != "":
		file, err := os.Open(filepath.Clean(f.modules))
		if err != nil {
			return nil, err
		}
		defer func() { _ = file.Close() }()
		var out []string
		s := bufio.NewScanner(file)
		for s.Scan() {
			if line := strings.TrimSpace(s.Text()); line != "" && !strings.HasPrefix(line, "#") {
				out = append(out, strings.ReplaceAll(line, ".", "/"))
			}
		}
		return out, s.Err()
	}
	return nil, fmt.Errorf("%w: give -modules or -mathlib", errUsage)
}

func runVolumes(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: give a subcommand: check, orphans or home", errUsage)
	}
	sub, args := args[0], args[1:]
	fs := flag.NewFlagSet("volumes "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f mapFlags
	f.register(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	m, err := f.volumes()
	if err != nil {
		return err
	}
	switch sub {
	case "check", "orphans":
		mods, err := f.moduleList()
		if err != nil {
			return err
		}
		r := m.Check(mods)
		if sub == "orphans" {
			for _, o := range r.Orphans {
				fmt.Fprintln(stdout, o)
			}
			if len(r.Orphans) > 0 {
				return errFailed
			}
			return nil
		}
		writeReport(stdout, m, r)
		if !r.OK() {
			return errFailed
		}
		return nil
	case "home":
		if fs.NArg() == 0 {
			return fmt.Errorf("%w: give one or more modules", errUsage)
		}
		for _, mod := range fs.Args() {
			mod = strings.ReplaceAll(mod, ".", "/")
			vol, p, ok := m.Home(mod)
			if !ok {
				fmt.Fprintf(stdout, "%s\tno home\n", mod)
				continue
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", mod, vol, p)
		}
		return nil
	}
	return fmt.Errorf("%w: unknown subcommand %q", errUsage, sub)
}

func writeReport(w io.Writer, m *blueprint.VolumeMap, r blueprint.Report) {
	fmt.Fprintf(w, "%d modules, %d volumes, %d orphans, %d unused prefixes, %d problems\n", r.Modules, len(m.Volumes), len(r.Orphans), len(r.Unused), len(r.Problems))
	ids := make([]string, 0, len(r.PerVol))
	for id := range r.PerVol {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(w, "  %s\t%d\n", id, r.PerVol[id])
	}
	for _, p := range r.Problems {
		fmt.Fprintln(w, "problem:", p)
	}
	for _, o := range r.Orphans {
		fmt.Fprintln(w, "orphan:", o)
	}
	for _, u := range r.Unused {
		fmt.Fprintln(w, "unused prefix:", u)
	}
}

// runCheck checks the files of the map. With -mathlib or -modules it also checks the volume map against the modules.
func runCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f mapFlags
	f.register(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	failed := false
	m, err := f.volumes()
	if err != nil {
		fmt.Fprintln(stdout, "blueprint/volumes.yaml:", err)
		failed = true
	} else {
		var mods []string
		if f.mathlib != "" || f.modules != "" {
			if mods, err = f.moduleList(); err != nil {
				return err
			}
		}
		r := m.Check(mods)
		if mods == nil {
			r.Unused = nil
		}
		fmt.Fprintf(stdout, "blueprint/volumes.yaml: %d volumes, %d problems", len(m.Volumes), len(r.Problems))
		if mods != nil {
			fmt.Fprintf(stdout, ", %d modules, %d orphans, %d unused prefixes", r.Modules, len(r.Orphans), len(r.Unused))
		}
		fmt.Fprintln(stdout)
		for _, p := range r.Problems {
			fmt.Fprintln(stdout, "  problem:", p)
		}
		for _, o := range r.Orphans {
			fmt.Fprintln(stdout, "  orphan:", o)
		}
		for _, u := range r.Unused {
			fmt.Fprintln(stdout, "  unused prefix:", u)
		}
		failed = failed || !r.OK()
	}
	ms, err := blueprint.LoadMilestones(filepath.Join(f.mapDir, "blueprint", "milestones.yaml"))
	if err != nil {
		fmt.Fprintln(stdout, "blueprint/milestones.yaml:", err)
		failed = true
	} else {
		probs := blueprint.CheckMilestones(ms)
		fmt.Fprintf(stdout, "blueprint/milestones.yaml: %d milestones, %d problems\n", len(ms), len(probs))
		for _, p := range probs {
			fmt.Fprintln(stdout, "  problem:", p)
		}
		failed = failed || len(probs) > 0
	}
	if _, err := os.Stat(familiesPath(f.mapDir)); err == nil {
		if fams, err := family.Load(familiesPath(f.mapDir)); err != nil {
			fmt.Fprintln(stdout, "blueprint/families.yaml:", err)
			failed = true
		} else {
			fmt.Fprintf(stdout, "blueprint/families.yaml: %d families, 0 problems\n", len(fams))
		}
	}
	files, _ := filepath.Glob(filepath.Join(f.mapDir, "informal", "*.jsonl"))
	for _, file := range files {
		rs, err := informal.Load(file)
		if err != nil {
			fmt.Fprintln(stdout, "informal:", err)
			failed = true
			continue
		}
		fmt.Fprintf(stdout, "informal/%s: %d records, 0 problems\n", filepath.Base(file), len(rs))
	}
	if eds, err := edition.Load(edition.Path(f.mapDir)); err != nil {
		fmt.Fprintln(stdout, "records/editions.json:", err)
		failed = true
	} else if eds != nil {
		fmt.Fprintf(stdout, "records/editions.json: %d editions in the log, 0 problems\n", len(eds))
	}
	if failed {
		return errFailed
	}
	return nil
}
