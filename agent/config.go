package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Config is agent.yaml of the map (spec 11.3, 11.6).
type Config struct {
	// Models maps a role, "default", "large" or "small" to a model.
	Models map[string]string `yaml:"models"`
	Budget struct {
		Day  float64            `yaml:"day"`
		Run  float64            `yaml:"run"`
		Role map[string]float64 `yaml:"role"`
	} `yaml:"budget"`
}

// SpecModels is the model table that mlib uses for the record when there is no agent.yaml. Then the budget is zero and no call goes out.
var SpecModels = map[string]string{
	"large": "claude-opus-5-5",
	"small": "claude-sonnet-5-5",
}

// LoadConfig reads agent.yaml of the map. It gives nil when the file is missing.
func LoadConfig(mapDir string) (*Config, error) {
	path := filepath.Join(mapDir, "agent.yaml")
	data, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if c.Budget.Day < 0 || c.Budget.Run < 0 {
		return nil, fmt.Errorf("%s: a budget is negative", path)
	}
	return &c, nil
}

// Entry is one line of agent/ledger.jsonl: one model call. The runner writes the ledger. mlib only reads it.
type Entry struct {
	Date          string  `json:"date,omitempty"` // YYYY-MM-DD; empty means the date of the run
	Run           string  `json:"run"`
	Role          string  `json:"role"`
	Model         string  `json:"model"`
	PromptVersion string  `json:"prompt_version"`
	TokensIn      int     `json:"tokens_in"`
	TokensOut     int     `json:"tokens_out"`
	Cost          float64 `json:"cost"` // dollars
}

// LedgerPath is the place of the ledger in the map.
func LedgerPath(mapDir string) string {
	return filepath.Join(mapDir, "agent", "ledger.jsonl")
}

// LoadLedger reads agent/ledger.jsonl. It gives nil when the file is missing. An entry with no date gets the date of its run.
func LoadLedger(mapDir string, runs []*Run) ([]Entry, error) {
	path := LedgerPath(mapDir)
	f, err := os.Open(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	dates := map[string]string{}
	for _, r := range runs {
		dates[r.ID] = r.Date
	}
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var e Entry
		dec := json.NewDecoder(strings.NewReader(text))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if e.Date == "" {
			e.Date = dates[e.Run]
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// Budget applies the limits of agent.yaml to the budget that the user asks for: the run limit, what is left of the day, and what is left of the day for the role. With no agent.yaml the budget is zero (spec 11.6). The note says which limits apply.
func Budget(cfg *Config, ledger []Entry, role, date string, asked float64) (float64, string) {
	if cfg == nil {
		return 0, "there is no agent.yaml, so the budget is zero and no call goes out"
	}
	b := asked
	var notes []string
	if b > cfg.Budget.Run {
		b = cfg.Budget.Run
		notes = append(notes, fmt.Sprintf("the run limit of agent.yaml is %.2f", cfg.Budget.Run))
	}
	day, dayRole := 0.0, 0.0
	for _, e := range ledger {
		if e.Date != date {
			continue
		}
		day += e.Cost
		if e.Role == role {
			dayRole += e.Cost
		}
	}
	if left := max(0, cfg.Budget.Day-day); b > left {
		b = left
		notes = append(notes, fmt.Sprintf("%.2f of the day limit %.2f is left", left, cfg.Budget.Day))
	}
	if limit, ok := cfg.Budget.Role[role]; ok {
		if left := max(0, limit-dayRole); b > left {
			b = left
			notes = append(notes, fmt.Sprintf("%.2f of the day limit %.2f of the role %s is left", left, limit, role))
		}
	}
	return b, strings.Join(notes, "; ")
}

// Model gives the model of a role and where it comes from. A role with a small model goes to the large model after two failed runs on the item (spec 11.3).
func Model(cfg *Config, role *Role, failed int) (string, string) {
	size := "small"
	if role.Large {
		size = "large"
	}
	escalate := !role.Large && failed >= 2
	if escalate {
		size = "large"
	}
	if cfg == nil {
		return SpecModels[size], "the " + size + " model of the default table; there is no agent.yaml"
	}
	if m := cfg.Models[role.Name]; m != "" && !escalate {
		return m, "agent.yaml, models." + role.Name
	}
	if m := cfg.Models[size]; m != "" {
		if escalate {
			return m, fmt.Sprintf("agent.yaml, models.large, after %d failed runs", failed)
		}
		return m, "agent.yaml, models." + size
	}
	if m := cfg.Models["default"]; m != "" {
		return m, "agent.yaml, models.default"
	}
	return "", "agent.yaml names no model for the role and no default"
}
