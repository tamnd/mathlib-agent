package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tamnd/mathlib-agent/agent"
)

func runAgent(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "run" {
		return fmt.Errorf("%w: give the subcommand run, for example mlib agent run informalizer -dry-run", errUsage)
	}
	return agentRun(args[1:], stdout, stderr)
}

func agentRun(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("agent run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	qf := addQueueFlags(fs)
	item := fs.String("item", "", "the item to work on: a volume, a module, a name shape, a section or a commit; the default is the top of the queue")
	budget := fs.Float64("budget", 5, "the budget of the run in dollars; agent.yaml can lower it")
	dryRun := fs.Bool("dry-run", false, "print the record and the prompt, and write nothing")
	timeout := fs.Duration("timeout", time.Hour, "the time limit of the runner")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: mlib agent run <role> [flags]")
		fmt.Fprintln(stderr, "The environment variable MLIB_RUNNER names the command that runs the prompt. It reads the prompt on stdin. Without it, the run stops after the record.")
		fs.PrintDefaults()
	}
	var roleName string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		roleName, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case roleName == "" && fs.NArg() == 1:
		roleName = fs.Arg(0)
	case roleName == "" || fs.NArg() > 0:
		return fmt.Errorf("%w: agent run needs one role: %s", errUsage, agent.RoleNames())
	}
	if *budget < 0 {
		return fmt.Errorf("%w: the budget is negative", errUsage)
	}
	role, err := agent.FindRole(roleName)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	in, err := loadInput(qf)
	if err != nil {
		return err
	}
	var it agent.Item
	switch {
	case *item != "":
		var ok bool
		if it, ok = in.Find(role, *item); !ok {
			return fmt.Errorf("the role %s has no item %s", role.Name, *item)
		}
	case role.Target == agent.TargetBump:
		return fmt.Errorf("%w: the porter needs the commit of the failing bump; give -item <commit>", errUsage)
	default:
		items := in.Queue(role)
		if len(items) == 0 {
			return fmt.Errorf("the queue of the role %s is empty", role.Name)
		}
		it = items[0]
	}
	cfg, err := agent.LoadConfig(*qf.mapDir)
	if err != nil {
		return err
	}
	ledger, err := agent.LoadLedger(*qf.mapDir, in.Runs)
	if err != nil {
		return err
	}
	day := in.Date.Format(time.DateOnly)
	attempts := agent.FailedRuns(in.Runs, role.Name, it.Target)
	model, from := agent.Model(cfg, role, len(attempts))
	b, note := agent.Budget(cfg, ledger, role.Name, day, *budget)
	if model == "" {
		b, note = 0, "agent.yaml names no model for the role, so no call goes out"
	}
	if role.Name == "checker" {
		if w := lastWriter(in.Runs, it.Target); w != nil && w.Model == model {
			return fmt.Errorf("the writer run %s on %s used the model %s; the writer and the checker never use the same model in one item (spec 11.3); set models.checker in agent.yaml to another model", w.ID, it.Target, model)
		}
	}
	id, branch := agent.NextID(*qf.mapDir, in.Runs, role.Name, it.Target, day)
	rec := &agent.Run{
		ID: id, Role: role.Name, Target: it.Target, Kind: it.Kind, Title: it.Title, Volume: it.Volume, Date: day, Status: agent.Planned,
		Model: model, ModelFrom: from, BudgetAsked: *budget, Budget: b, BudgetNote: note, Branch: branch,
		Rank: it.Rank, Reason: it.Reason(), Checks: role.Checked, Mathlib: in.Index.Header.Mathlib, Attempts: attempts,
		Tool: "mlib", ToolVersion: Version,
	}
	prompt, version, err := agent.Prompt(in, role, it, rec, *qf.mapDir)
	if err != nil {
		return err
	}
	rec.PromptVersion, rec.PromptSHA256 = version, agent.Hash(prompt)
	runner := os.Getenv("MLIB_RUNNER")
	switch {
	case *dryRun:
		rec.Note = "a dry run: mlib wrote nothing and started no runner"
	case b <= 0:
		rec.Note = "the budget is zero, so mlib did not start a runner: " + note
	case runner == "":
		rec.Note = "no runner: MLIB_RUNNER is not set, so the run stops after this record"
	}
	if *dryRun {
		out, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\n\n%s", out, prompt)
		return nil
	}
	recPath, err := agent.Write(*qf.mapDir, rec, prompt)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s: the role %s on %s, status planned, model %s, budget %.2f dollars\n", recPath, role.Name, it.Target, model, b)
	if b <= 0 || runner == "" {
		fmt.Fprintf(stdout, "The run stops here: %s.\n", rec.Note)
		return nil
	}
	if err := agent.Start(runner, *timeout, *qf.mapDir, recPath, rec, prompt); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "the runner %s exited with %d after %.1f s; status %s\n", rec.Runner.Command, rec.Runner.Exit, rec.Runner.Seconds, rec.Status)
	if rec.Status == agent.Failed {
		return fmt.Errorf("the run %s failed: %s", rec.ID, rec.Note)
	}
	return nil
}

// lastWriter gives the last writer run on a section that ran or passed.
func lastWriter(runs []*agent.Run, target string) *agent.Run {
	var out *agent.Run
	for _, r := range runs {
		if r.Role == "writer" && r.Target == target && (r.Status == agent.Ran || r.Status == agent.Passed) {
			out = r
		}
	}
	return out
}
