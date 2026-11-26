// Package agent ranks the work of the roles of spec 11.3 and runs one role at a time within the budget of agent.yaml. It writes a run record for each run and starts an outside runner with the prompt. It never decides that a run passed: Lean, the checks of spec 14 and people decide that.
package agent

import (
	"fmt"
	"strings"
)

// The kinds of target that a queue holds.
const (
	TargetVolume  = "volume"
	TargetModule  = "module"
	TargetGroup   = "group"
	TargetSection = "section"
	TargetBump    = "bump"
)

// Role is one row of the role table of spec 11.3.
type Role struct {
	Name    string
	Input   string
	Output  string
	Checked string
	// Target is the kind of item in the queue of the role.
	Target string
	// Large is true for a role that uses the large model by default. A role with a small model goes to the large model after two failed runs on the same item.
	Large bool
}

// Roles is the role table of spec 11.3.
var Roles = []Role{
	{"planner", "a volume of 06 and the modules of its prefixes", "the section plan: sections, the modules of each, the E3 list, the requires edges", "a person accepts the plan", TargetVolume, true},
	{"informalizer", "one module: its declarations with types, docstrings and the module docstring", "one E1 sentence per declaration that has no record or a stale one", "the round trip and the critic (14.3); a person samples", TargetModule, false},
	{"family", "the declarations with no family and depth E1, in groups by name shape", "proposed family patterns and texts", "a person accepts each family", TargetGroup, false},
	{"writer", "a section plan, the declarations with their types and docstrings, the module docstrings, the E1 records, the model section of 07.9", "the section text, parts 1 to 7", "mlhb check; the checker; a person", TargetSection, true},
	{"exampler", "a drafted section", "the Lean examples with anchors, in Examples.lean", "lake build", TargetSection, false},
	{"exerciser", "a drafted section", "the exercises, the (L) statement and solution files, the solutions appendix text", "lake build; the statement hash check of 10.4", TargetSection, false},
	{"checker", "a section, with its declarations at the edition commit", "a list of faults: a claim that does not match a Lean type, a missing hypothesis, a proof gap, a wrong reference, a style fault", "a person reads the list; the writer fixes", TargetSection, true},
	{"refresher", "a stale section and its drift items", "a minimal diff that makes the section true at the new commit", "mlhb check; the checker; a person for a breaking item", TargetSection, true},
	{"porter", "a failing bump of the companion", "fixes to the examples and solutions", "lake build", TargetBump, false},
}

// FindRole gives the role with the name.
func FindRole(name string) (*Role, error) {
	for i := range Roles {
		if Roles[i].Name == name {
			return &Roles[i], nil
		}
	}
	return nil, fmt.Errorf("unknown role %q; use one of %s", name, RoleNames())
}

// RoleNames gives the names of the roles, with commas.
func RoleNames() string {
	var ns []string
	for _, r := range Roles {
		ns = append(ns, r.Name)
	}
	return strings.Join(ns, ", ")
}
