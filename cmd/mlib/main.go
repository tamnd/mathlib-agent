// Command mlib is the toolchain of The Mathlib Handbook. It reads the index of Mathlib that tamnd/mathlib-lean writes, computes the depth of each declaration, the coverage and the drift, checks the map of tamnd/mathlib, ranks the work of the roles and builds the site mathlib.tamnd.com.
//
// Exit codes: 0 for success, 1 for a failed check or a failed run, 2 for a usage error, and 3 for a command that is not written yet.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

// Version is the version of mlib. The release build sets it with -ldflags "-X main.Version=v0.1.0".
var Version = "dev"

// errUsage is a usage error. main exits with code 2.
var errUsage = errors.New("usage error")

// command is one subcommand of mlib.
type command struct {
	name      string
	summary   string
	milestone string // the milestone that adds the command, when it is not written yet
	run       func(args []string, stdout, stderr io.Writer) error
}

// commands returns the subcommands of mlib, in the order of spec 11.1.
func commands() []command {
	return []command{
		{name: "version", summary: "print the version of mlib", run: runVersion},
		{name: "index", summary: "download or read the index of Mathlib at a commit", run: runIndex},
		{name: "sync", summary: "compare a new index with the last one and write the drift, the depths and the modules", run: runSync},
		{name: "coverage", summary: "compute the coverage numbers of spec 04.5", run: runCoverage},
		{name: "depth", summary: "show the computed depth of a declaration and its reason", run: runDepth},
		{name: "family", summary: "match, list or show the E0 families", milestone: "M1"},
		{name: "volumes", summary: "check the volume map: modules with no home, prefixes with no module", run: runVolumes},
		{name: "plan", summary: "run the planner role on a volume", milestone: "M1"},
		{name: "informal", summary: "the E1 records and their checks", milestone: "M1"},
		{name: "queue", summary: "print the work queue of a role, with the reason of each rank", milestone: "M1"},
		{name: "agent", summary: "run one role within the budget", milestone: "M1"},
		{name: "rename", summary: "rewrite deprecated names in the handbook and the companion", milestone: "M1"},
		{name: "edition", summary: "open, list or show an edition", milestone: "M0"},
		{name: "report", summary: "write the weekly or the edition report", milestone: "M1"},
		{name: "site", summary: "build, check or serve the site", milestone: "M0"},
		{name: "check", summary: "check the map: the volume map, the families, the records and the books", run: runCheck},
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs mlib with the arguments args and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "-help" || args[0] == "--help" {
		usage(stdout)
		return 0
	}
	if args[0] == "-version" || args[0] == "--version" {
		args = []string{"version"}
	}
	for _, c := range commands() {
		if c.name != args[0] {
			continue
		}
		if c.run == nil {
			fmt.Fprintf(stderr, "mlib %s: this command is not written yet. Milestone %s adds it.\n", c.name, c.milestone)
			return 3
		}
		err := c.run(args[1:], stdout, stderr)
		switch {
		case err == nil, errors.Is(err, flag.ErrHelp):
			return 0
		case errors.Is(err, errUsage):
			fmt.Fprintf(stderr, "mlib %s: %v\n", c.name, err)
			return 2
		default:
			fmt.Fprintf(stderr, "mlib %s: %v\n", c.name, err)
			return 1
		}
	}
	fmt.Fprintf(stderr, "mlib: unknown command %q. Run mlib help to see the commands.\n", args[0])
	return 2
}

// usage writes the help text.
func usage(w io.Writer) {
	fmt.Fprintln(w, "mlib is the toolchain of The Mathlib Handbook: read the index of Mathlib, check the map and build mathlib.tamnd.com.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: mlib <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	cs := commands()
	width := 0
	for _, c := range cs {
		width = max(width, len(c.name))
	}
	for _, c := range cs {
		note := ""
		if c.run == nil {
			note = " (not written yet, milestone " + c.milestone + ")"
		}
		fmt.Fprintf(w, "  %-*s  %s%s\n", width, c.name, c.summary, note)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run mlib <command> -h to see the flags of a command.")
}

func runVersion(args []string, stdout, _ io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("%w: version takes no arguments", errUsage)
	}
	fmt.Fprintln(stdout, "mlib", Version)
	return nil
}

// names returns the names of the commands, sorted. The tests use it.
func names() []string {
	var out []string
	for _, c := range commands() {
		out = append(out, c.name)
	}
	sort.Strings(out)
	return out
}
