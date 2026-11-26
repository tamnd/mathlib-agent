package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The statuses of a run record. A run starts as planned. The runner makes it ran or failed. A person or the checks of the role set passed. mlib never sets passed from the output of a model (spec 14.3).
const (
	Planned = "planned"
	Ran     = "ran"
	Failed  = "failed"
	Passed  = "passed"
)

// Run is records/runs/<id>/run.json, the record of one run of a role.
type Run struct {
	ID            string   `json:"id"`
	Role          string   `json:"role"`
	Target        string   `json:"target"`
	Kind          string   `json:"kind"`
	Title         string   `json:"title"`
	Volume        string   `json:"volume,omitempty"`
	Date          string   `json:"date"`
	Status        string   `json:"status"`
	Model         string   `json:"model"`
	ModelFrom     string   `json:"model_from"`
	PromptVersion string   `json:"prompt_version"`
	PromptSHA256  string   `json:"prompt_sha256"`
	BudgetAsked   float64  `json:"budget_asked"`
	Budget        float64  `json:"budget"`
	BudgetNote    string   `json:"budget_note,omitempty"`
	Branch        string   `json:"branch"`
	Rank          float64  `json:"rank"`
	Reason        string   `json:"reason"`
	Checks        string   `json:"checks"`
	Mathlib       string   `json:"mathlib"`
	Attempts      []string `json:"attempts,omitempty"`
	Runner        *Runner  `json:"runner,omitempty"`
	PullRequest   string   `json:"pull_request,omitempty"`
	Note          string   `json:"note,omitempty"`
	Tool          string   `json:"tool"`
	ToolVersion   string   `json:"tool_version"`
}

// Runner is the outside command that ran the prompt.
type Runner struct {
	Command string  `json:"command"` // the base name of the command, so the record has no local path
	Exit    int     `json:"exit"`
	Output  string  `json:"output"`
	Started string  `json:"started"`
	Seconds float64 `json:"seconds"`
}

// RunsDir is the directory of the run records in the map.
func RunsDir(mapDir string) string {
	return filepath.Join(mapDir, "records", "runs")
}

// LoadRuns reads records/runs/<id>/run.json, in id order. A missing directory has no runs.
func LoadRuns(mapDir string) ([]*Run, error) {
	dir := RunsDir(mapDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Run
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "run.json")
		data, err := os.ReadFile(filepath.Clean(path))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var r Run
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&r); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, &r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

var unsafeID = regexp.MustCompile(`[^A-Za-z0-9.-]+`)

// Slug makes a target safe for a run id and a branch name.
func Slug(s string) string {
	s = strings.Trim(unsafeID.ReplaceAllString(s, "-"), "-")
	if len(s) > 60 {
		s = s[:60]
	}
	if s == "" {
		s = "item"
	}
	return s
}

// NextID gives the id and the branch of the next run of a role on a target: the date, the role, the target and a number, so that two runs of one day differ.
func NextID(mapDir string, runs []*Run, role, target, date string) (id, branch string) {
	slug := Slug(target)
	base := date + "-" + role + "-" + slug
	seq := 1
	for _, r := range runs {
		if strings.HasPrefix(r.ID, base+"-") {
			seq++
		}
	}
	for {
		if _, err := os.Stat(filepath.Join(RunsDir(mapDir), fmt.Sprintf("%s-%d", base, seq))); errors.Is(err, fs.ErrNotExist) {
			break
		}
		seq++
	}
	return fmt.Sprintf("%s-%d", base, seq), fmt.Sprintf("agent/%s/%s-%s-%d", role, slug, strings.ReplaceAll(date, "-", ""), seq)
}

// FailedRuns gives the ids of the failed runs of a role on a target.
func FailedRuns(runs []*Run, role, target string) []string {
	var out []string
	for _, r := range runs {
		if r.Role == role && r.Target == target && r.Status == Failed {
			out = append(out, r.ID)
		}
	}
	return out
}

// DataBlock puts text from Mathlib or from an earlier run in a marked block (spec 11.4). The marker has a hash of the text, so the text cannot close the block.
func DataBlock(label, text string) string {
	sum := sha256.Sum256([]byte(text))
	tag := hex.EncodeToString(sum[:])[:12]
	return fmt.Sprintf("<<<DATA %s %s\n%s\nDATA %s>>>\n", tag, label, strings.TrimRight(text, "\n"), tag)
}

// Hash gives the SHA-256 of a prompt in hex, for the record.
func Hash(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// Write writes the run record to records/runs/<id>/run.json and the prompt to prompt.md beside it. It gives the path of the record.
func Write(mapDir string, r *Run, prompt string) (string, error) {
	r.PromptSHA256 = Hash(prompt)
	dir := filepath.Join(RunsDir(mapDir), r.ID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o600); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "run.json")
	return path, save(path, r)
}

func save(path string, r *Run) error {
	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o600)
}

// Start runs the outside command with the prompt on stdin, in the directory of the map. Its output goes to output.log as data; mlib does not read instructions from it. The record gets the exit status and the status ran or failed.
func Start(runner string, timeout time.Duration, mapDir, recPath string, r *Run, prompt string) error {
	dir := filepath.Dir(recPath)
	logPath := filepath.Join(dir, "output.log")
	logf, err := os.OpenFile(filepath.Clean(logPath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, runner) // #nosec G204 -- the runner is a command that the user names in MLIB_RUNNER
	cmd.Dir = mapDir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.Env = append(os.Environ(),
		"MLIB_RUN_ID="+r.ID, "MLIB_ROLE="+r.Role, "MLIB_TARGET="+r.Target, "MLIB_MODEL="+r.Model,
		fmt.Sprintf("MLIB_BUDGET=%.2f", r.Budget), "MLIB_BRANCH="+r.Branch, "MLIB_RUN_DIR="+dir, "MLIB_RECORD="+recPath, "MLIB_MATHLIB="+r.Mathlib)
	start := time.Now()
	runErr := cmd.Run()
	if err := logf.Close(); err != nil && runErr == nil {
		runErr = err
	}
	rr := &Runner{Command: filepath.Base(runner), Output: "output.log", Started: start.UTC().Format(time.RFC3339), Seconds: time.Since(start).Seconds()}
	r.Runner = rr
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		r.Status = Ran
		r.Note = "the runner exited with 0; the result waits for the checks of the role: " + r.Checks
	case errors.As(runErr, &exitErr):
		rr.Exit = exitErr.ExitCode()
		r.Status = Failed
		r.Note = fmt.Sprintf("the runner exited with %d", rr.Exit)
	default:
		rr.Exit = -1
		r.Status = Failed
		r.Note = "the runner did not run: " + runErr.Error()
	}
	if ctx.Err() != nil {
		r.Status = Failed
		r.Note = "the runner passed the time limit of " + timeout.String()
	}
	return save(recPath, r)
}
