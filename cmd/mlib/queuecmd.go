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
	"text/tabwriter"
	"time"

	"github.com/tamnd/mathlib-agent/agent"
	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/family"
	"github.com/tamnd/mathlib-agent/informal"
	"github.com/tamnd/mathlib-agent/plan"
)

// queueFlags are the flags that mlib queue and mlib agent run share.
type queueFlags struct {
	mapDir, dir, lists, milestone, date *string
}

func addQueueFlags(fs *flag.FlagSet) queueFlags {
	return queueFlags{
		mapDir:    fs.String("map", ".", "the checkout of tamnd/mathlib"),
		dir:       fs.String("index", "index", "the directory of the index"),
		lists:     fs.String("lists", "", "a Mathlib checkout with the curated lists in docs/; the default is the index directory"),
		milestone: fs.String("milestone", "", "the current milestone, for the weight of a volume; the default is the first milestone that is not closed"),
		date:      fs.String("date", "", "the date for the age factor and the run, YYYY-MM-DD; the default is today, UTC"),
	}
}

// loadInput reads the index, the map and the run records for the queues.
func loadInput(f queueFlags) (*agent.Input, error) {
	day := time.Now().UTC()
	if *f.date != "" {
		t, err := time.Parse(time.DateOnly, *f.date)
		if err != nil {
			return nil, fmt.Errorf("%w: the date %q is not YYYY-MM-DD", errUsage, *f.date)
		}
		day = t
	}
	vm, err := blueprint.LoadVolumes(filepath.Join(*f.mapDir, "blueprint", "volumes.yaml"))
	if err != nil {
		return nil, err
	}
	ms, err := blueprint.LoadMilestones(filepath.Join(*f.mapDir, "blueprint", "milestones.yaml"))
	if err != nil {
		return nil, err
	}
	cur := *f.milestone
	switch {
	case cur != "" && blueprint.Find(ms, cur) == nil:
		return nil, fmt.Errorf("%w: blueprint/milestones.yaml has no milestone %s", errUsage, cur)
	case cur == "":
		if m := blueprint.Current(ms); m != nil {
			cur = m.ID
		}
	}
	ix, depths, _, err := loadDepths(*f.dir, *f.lists)
	if err != nil {
		return nil, err
	}
	in := &agent.Input{Index: ix, Volumes: vm, Milestones: ms, Current: cur, Depths: depths, Date: day, Plans: map[string]*plan.Plan{}, Informal: map[string][]informal.Record{}}
	if _, err := os.Stat(familiesPath(*f.mapDir)); err == nil {
		fams, err := family.Load(familiesPath(*f.mapDir))
		if err != nil {
			return nil, err
		}
		in.Families = family.Match(ix, fams).Member
	}
	for _, v := range vm.Volumes {
		p, err := plan.Load(plan.Path(*f.mapDir, v.ID))
		switch {
		case err == nil:
			in.Plans[v.ID] = p
		case !errors.Is(err, fs.ErrNotExist):
			return nil, err
		}
		rs, err := informal.Load(informal.Path(*f.mapDir, v.ID))
		if err != nil {
			return nil, err
		}
		in.Informal[v.ID] = rs
	}
	in.Runs, err = agent.LoadRuns(*f.mapDir)
	return in, err
}

func runQueue(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("queue", flag.ContinueOnError)
	fs.SetOutput(stderr)
	qf := addQueueFlags(fs)
	roleName := fs.String("role", "", "the role: "+agent.RoleNames())
	n := fs.Int("n", 20, "the number of items to print; 0 prints all")
	asJSON := fs.Bool("json", false, "print the items as JSON lines")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *roleName == "" {
		return fmt.Errorf("%w: give -role, for example mlib queue -role writer", errUsage)
	}
	if *n < 0 {
		return fmt.Errorf("%w: -n is negative", errUsage)
	}
	role, err := agent.FindRole(*roleName)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	in, err := loadInput(qf)
	if err != nil {
		return err
	}
	items := in.Queue(role)
	total := len(items)
	if *n > 0 && len(items) > *n {
		items = items[:*n]
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		for _, it := range items {
			if err := enc.Encode(it); err != nil {
				return err
			}
		}
		return nil
	}
	fmt.Fprintf(stdout, "The queue of the role %s: %d items, milestone %s.\n", role.Name, total, in.Current)
	fmt.Fprintln(stdout, "rank = weight x (1 + readers that wait) x level factor / (1 + failed runs) x age factor (spec 11.5)")
	if total == 0 {
		if role.Target == agent.TargetBump {
			fmt.Fprintln(stdout, "The porter works on a failing bump. Give the commit with mlib agent run porter -item <commit>.")
		}
		return nil
	}
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "rank\ttarget\tvolume\ttitle\twhy")
	for _, it := range items {
		fmt.Fprintf(tw, "%.3f\t%s\t%s\t%s\t%s\n", it.Rank, it.Target, it.Volume, it.Title, it.Reason())
	}
	return tw.Flush()
}
