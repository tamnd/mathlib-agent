package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 0 {
		t.Fatalf("help exit code %d", code)
	}
	for _, n := range names() {
		if !strings.Contains(out.String(), "  "+n+" ") {
			t.Errorf("help does not list %s", n)
		}
	}
}

func TestVersion(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"version"}, &out, &errb); code != 0 {
		t.Fatalf("version exit code %d", code)
	}
	if got := out.String(); got != "mlib dev\n" {
		t.Errorf("version printed %q", got)
	}
	if code := run([]string{"version", "x"}, &out, &errb); code != 2 {
		t.Errorf("version with an argument: exit code %d, want 2", code)
	}
}

func TestUnknown(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"frobnicate"}, &out, &errb); code != 2 {
		t.Errorf("unknown command: exit code %d, want 2", code)
	}
}

func TestNotWritten(t *testing.T) {
	for _, c := range commands() {
		if c.run != nil {
			continue
		}
		if c.milestone == "" {
			t.Errorf("%s is not written and has no milestone", c.name)
		}
		var out, errb bytes.Buffer
		if code := run([]string{c.name}, &out, &errb); code != 3 {
			t.Errorf("%s: exit code %d, want 3", c.name, code)
		}
	}
}

func TestQueueAndAgentUsage(t *testing.T) {
	for _, args := range [][]string{
		{"queue"},
		{"queue", "-role", "nobody"},
		{"queue", "-role", "writer", "-n", "-1"},
		{"agent"},
		{"agent", "run"},
		{"agent", "run", "nobody"},
		{"agent", "run", "writer", "-budget", "-1"},
	} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 2 {
			t.Errorf("%v: exit code %d, want 2; %s", args, code, errb.String())
		}
	}
}
