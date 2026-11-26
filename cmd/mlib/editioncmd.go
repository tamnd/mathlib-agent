package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tamnd/mathlib-agent/edition"
)

const leanRepo = "tamnd/mathlib-lean"

// commitOf gives the commit sha and the committer date of a ref of a repository.
func commitOf(ctx context.Context, repo, ref string) (sha, date string, err error) {
	body, err := get(ctx, "https://api.github.com/repos/"+repo+"/commits/"+ref, "application/vnd.github+json")
	if err != nil {
		return "", "", err
	}
	defer func() { _ = body.Close() }()
	var c struct {
		SHA    string `json:"sha"`
		Commit struct {
			Committer struct {
				Date time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(body).Decode(&c); err != nil {
		return "", "", err
	}
	return c.SHA, c.Commit.Committer.Date.UTC().Format(time.DateOnly), nil
}

// toolchainOf gives the Lean version of the lean-toolchain file of Mathlib at a ref, for example v4.36.0.
func toolchainOf(ctx context.Context, ref string) (string, error) {
	body, err := get(ctx, "https://raw.githubusercontent.com/"+mathlibRepo+"/"+ref+"/lean-toolchain", "")
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()
	b, err := io.ReadAll(io.LimitReader(body, 1<<10))
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(b))
	return s[strings.LastIndex(s, ":")+1:], nil
}

func runEdition(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: give a subcommand: new, list or show", errUsage)
	}
	sub, args := args[0], args[1:]
	fs := flag.NewFlagSet("edition "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	mapDir := fs.String("map", ".", "the checkout of tamnd/mathlib")
	switch sub {
	case "new":
		tag := fs.String("tag", "", "the stable Mathlib tag of the edition, for example v4.36.0")
		date := fs.String("date", time.Now().UTC().Format(time.DateOnly), "the date the edition opens")
		notes := fs.String("notes", "", "a note for the log")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if !edition.TagRe.MatchString(*tag) {
			return fmt.Errorf("%w: the tag %q is not of the form v4.X.0; a patch tag does not open an edition", errUsage, *tag)
		}
		path := edition.Path(*mapDir)
		log, err := edition.Load(path)
		if err != nil {
			return err
		}
		ctx := context.Background()
		sha, tagDate, err := commitOf(ctx, mathlibRepo, *tag)
		if err != nil {
			return fmt.Errorf("the tag %s of %s: %w", *tag, mathlibRepo, err)
		}
		lean, err := toolchainOf(ctx, sha)
		if err != nil {
			return err
		}
		branch := "edition/" + *tag
		leanSHA, _, err := commitOf(ctx, leanRepo, branch)
		if err != nil {
			return fmt.Errorf("the branch %s of %s: %w; make the branch first", branch, leanRepo, err)
		}
		r := edition.Record{Edition: *tag, Mathlib: sha, Lean: lean, TagDate: tagDate, Opened: *date, MathlibLean: leanSHA, Notes: *notes}
		log, skipped, err := log.Add(r)
		if err != nil {
			return err
		}
		if err := log.Save(path); err != nil {
			return err
		}
		days, _ := edition.DaysBetween(tagDate, *date)
		fmt.Fprintf(stdout, "opened the edition %s: Mathlib %s, Lean %s, tag of %s, %d days ago, %s at %s\n", *tag, short(sha), lean, tagDate, days, branch, short(leanSHA))
		if skipped != "" {
			fmt.Fprintf(stdout, "the edition %s closed as skipped\n", skipped)
		}
		return nil
	case "list":
		if err := fs.Parse(args); err != nil {
			return err
		}
		log, err := edition.Load(edition.Path(*mapDir))
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "edition\tstate\tMathlib\ttag date\topened\treleased")
		for _, r := range log {
			rel := r.Released
			if rel == "" {
				rel = "-"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Edition, r.State, short(r.Mathlib), r.TagDate, r.Opened, rel)
		}
		return tw.Flush()
	case "show":
		if err := fs.Parse(args); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("%w: give one tag, for example mlib edition show v4.36.0", errUsage)
		}
		log, err := edition.Load(edition.Path(*mapDir))
		if err != nil {
			return err
		}
		r, ok := log.Find(fs.Arg(0))
		if !ok {
			return fmt.Errorf("the log has no edition %s", fs.Arg(0))
		}
		b, err := json.MarshalIndent(r, "", " ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s\n", b)
		return err
	}
	return fmt.Errorf("%w: unknown subcommand %q", errUsage, sub)
}
