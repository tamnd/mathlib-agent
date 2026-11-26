// Package prompts holds the prompts of the roles of spec 11.3, one file prompts/<role>.md each. The front matter of a file gives the role and the version of the prompt. The run record keeps the version, so that a result can be traced to the prompt that made it.
package prompts

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.md
var files embed.FS

// Prompt is one prompt file.
type Prompt struct {
	Role    string
	Version string
	Body    string
}

// Get gives the prompt of a role.
func Get(role string) (Prompt, error) {
	b, err := files.ReadFile(role + ".md")
	if err != nil {
		return Prompt{}, fmt.Errorf("there is no prompt for the role %s", role)
	}
	return Parse(string(b))
}

// Parse reads the front matter and the body of a prompt file.
func Parse(s string) (Prompt, error) {
	rest, ok := strings.CutPrefix(s, "---\n")
	if !ok {
		return Prompt{}, fmt.Errorf("the prompt has no front matter")
	}
	head, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return Prompt{}, fmt.Errorf("the front matter of the prompt has no end")
	}
	var p Prompt
	for _, line := range strings.Split(head, "\n") {
		k, v, _ := strings.Cut(line, ":")
		switch strings.TrimSpace(k) {
		case "role":
			p.Role = strings.TrimSpace(v)
		case "version":
			p.Version = strings.TrimSpace(v)
		}
	}
	if p.Role == "" || p.Version == "" {
		return Prompt{}, fmt.Errorf("the front matter of the prompt needs role and version")
	}
	p.Body = strings.TrimLeft(body, "\n")
	return p, nil
}
