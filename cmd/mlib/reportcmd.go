package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/mathlib-agent/report"
)

func runReport(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("report", flag.ContinueOnError)
	fl.SetOutput(stderr)
	mapDir := fl.String("map", ".", "the checkout of tamnd/mathlib")
	week := fl.String("week", "", "write the weekly report of an ISO week, for example 2027-W03; \"last\" is the last full week")
	ed := fl.String("edition", "", "write the report of an edition, for example v4.30.0")
	write := fl.Bool("write", false, "write the report to reports/weekly/<week>.md or reports/editions/<tag>.md; without it, print the report")
	if err := fl.Parse(args); err != nil {
		return err
	}
	if (*week == "") == (*ed == "") {
		return fmt.Errorf("%w: give -week or -edition, for example mlib report -week 2027-W03", errUsage)
	}
	in, err := report.Load(*mapDir, "mlib "+Version)
	if err != nil {
		return err
	}
	var text, path string
	if *week != "" {
		if *week == "last" {
			*week = report.WeekOf(time.Now().UTC().AddDate(0, 0, -7))
		}
		text, err = report.Weekly(in, *week)
		path = report.Path(*mapDir, "weekly", *week)
	} else {
		text, err = report.Edition(in, *ed)
		path = report.Path(*mapDir, "editions", *ed)
	}
	if err != nil {
		return err
	}
	if !*write {
		_, err = io.WriteString(stdout, text)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "wrote", path)
	return nil
}
