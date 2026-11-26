package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/informal"
)

func runInformal(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: give a subcommand: status, stale or sample", errUsage)
	}
	sub, args := args[0], args[1:]
	fs := flag.NewFlagSet("informal "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	mapDir := fs.String("map", ".", "the checkout of tamnd/mathlib")
	dir := fs.String("index", "index", "the directory of the index")
	lists := fs.String("lists", "", "a Mathlib checkout with the curated lists in docs/; the default is the index directory")
	vols := fs.String("volume", "", "the volumes, with commas; the default is every volume")
	n := fs.Int("n", 0, "sample: the number of records; the default is 1% of the records, at least 1 (spec 14.3)")
	seed := fs.Uint64("seed", 1, "sample: the seed of the random choice, so that a sample can be made again")
	switch sub {
	case "status", "stale", "sample":
	default:
		return fmt.Errorf("%w: unknown subcommand %q", errUsage, sub)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	vm, err := blueprint.LoadVolumes(filepath.Join(*mapDir, "blueprint", "volumes.yaml"))
	if err != nil {
		return err
	}
	var ids []string
	if *vols != "" {
		for _, v := range strings.Split(*vols, ",") {
			ids = append(ids, strings.TrimSpace(v))
		}
	} else {
		for _, v := range vm.Volumes {
			ids = append(ids, v.ID)
		}
	}
	ix, depths, _, err := loadDepths(*dir, *lists)
	if err != nil {
		return err
	}
	need := map[string]map[string]bool{}
	if sub == "status" {
		for i := range ix.Decls {
			d := &ix.Decls[i]
			if d.Deprecated != nil || depths[d.Name].Depth != depth.E1 {
				continue
			}
			if v, _, ok := vm.Home(d.Path()); ok {
				if need[v] == nil {
					need[v] = map[string]bool{}
				}
				need[v][d.Name] = true
			}
		}
	}
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	if sub == "status" {
		fmt.Fprintln(tw, "volume\tE1\tfresh\tstale\tmissing\tshare\tround trip\tcritic\thuman")
	}
	stale := 0
	for _, v := range ids {
		rs, err := informal.Load(informal.Path(*mapDir, v))
		if err != nil {
			return err
		}
		switch sub {
		case "status":
			s := informal.Compute(v, ix, need[v], rs)
			fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%.1f%%\t%s\t%s\t%s\n", v, s.Needed, s.Fresh, s.Stale, s.Missing, 100*s.Share(), passes(s.Roundtrip), passes(s.Critic), passes(s.Human))
		case "stale":
			for _, s := range informal.FindStale(ix, rs) {
				stale++
				fmt.Fprintf(tw, "%s\t%s\t%s\n", v, s.Record.Name, s.Reason)
			}
		case "sample":
			k := *n
			if k == 0 {
				k = max(1, len(rs)/100)
			}
			enc := json.NewEncoder(stdout)
			for _, r := range informal.Sample(rs, k, *seed) {
				if err := enc.Encode(r); err != nil {
					return err
				}
			}
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if sub == "stale" && stale > 0 {
		fmt.Fprintf(stdout, "%d stale records\n", stale)
		return errFailed
	}
	return nil
}

// passes formats the pass and fail counts of a check.
func passes(m map[string]int) string {
	return fmt.Sprintf("%d/%d", m[informal.Pass], m[informal.Pass]+m[informal.Fail])
}
