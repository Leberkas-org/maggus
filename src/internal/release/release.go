package release

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Commit represents a parsed git commit.
type Commit struct {
	Hash       string
	Subject    string
	Body       string
	Type       string // feat, fix, chore, etc.
	Scope      string // optional scope from parenthetical
	IsBreaking bool   // from ! suffix or BREAKING CHANGE: in body
}

// conventionalRe matches: type[(scope)][!]: description
var conventionalRe = regexp.MustCompile(`^(\w+)(?:\(([^)]*)\))?(!)?\s*:\s*(.*)$`)

// typeLabels maps conventional commit types to changelog section headers.
var typeLabels = map[string]string{
	"feat":     "Features",
	"fix":      "Bug Fixes",
	"docs":     "Documentation",
	"style":    "Styles",
	"refactor": "Code Refactoring",
	"perf":     "Performance Improvements",
	"test":     "Tests",
	"build":    "Build System",
	"ci":       "Continuous Integration",
	"chore":    "Chores",
	"revert":   "Reverts",
}

// sectionOrder defines the display order of changelog sections.
var sectionOrder = []string{
	"feat", "fix", "docs", "style", "refactor", "perf",
	"test", "build", "ci", "chore", "revert",
}

// FindLastTag runs git describe to find the most recent version tag.
// Returns empty string if no tags exist.
func FindLastTag(dir string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// No tags exist — not an error for our purposes.
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// commitSeparator is used to delimit fields in git log output.
const commitSeparator = "<<FIELD>>"

// logFormat is the git log pretty format.
// Fields: hash, subject, body
const logFormat = "%H" + commitSeparator + "%s" + commitSeparator + "%b"

// recordSeparator delimits individual commits in git log output.
const recordSeparator = "<<RECORD>>"

// CommitsSinceTag returns parsed commits from the given tag to HEAD.
// If tag is empty, returns all commits.
func CommitsSinceTag(dir, tag string) ([]Commit, error) {
	args := []string{"log", "--pretty=format:" + logFormat + recordSeparator}
	if tag != "" {
		args = append(args, tag+"..HEAD")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	records := strings.Split(raw, recordSeparator)
	var commits []Commit
	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, commitSeparator, 3)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		subject := parts[1]
		body := ""
		if len(parts) == 3 {
			body = strings.TrimSpace(parts[2])
		}

		c := Commit{
			Hash:    hash,
			Subject: subject,
			Body:    body,
		}
		parseConventional(&c)
		commits = append(commits, c)
	}
	return commits, nil
}

// parseConventional extracts type, scope, and breaking change info from the commit subject.
func parseConventional(c *Commit) {
	m := conventionalRe.FindStringSubmatch(c.Subject)
	if m == nil {
		return
	}
	c.Type = strings.ToLower(m[1])
	c.Scope = m[2]
	if m[3] == "!" {
		c.IsBreaking = true
	}
	if !c.IsBreaking && strings.Contains(c.Body, "BREAKING CHANGE:") {
		c.IsBreaking = true
	}
}

// GroupByType groups commits by their conventional commit type.
// Commits without a type are grouped under the empty string key.
func GroupByType(commits []Commit) map[string][]Commit {
	groups := make(map[string][]Commit)
	for _, c := range commits {
		groups[c.Type] = append(groups[c.Type], c)
	}
	return groups
}

// FormatChangelog formats grouped commits into a markdown changelog.
// The tag parameter is used in the heading; if empty, "Unreleased" is used.
func FormatChangelog(groups map[string][]Commit, tag string) string {
	var sb strings.Builder

	heading := tag
	if heading == "" {
		heading = "Unreleased"
	}
	fmt.Fprintf(&sb, "## %s\n", heading)

	// Render known types in order.
	for _, t := range sectionOrder {
		commits, ok := groups[t]
		if !ok {
			continue
		}
		label := typeLabels[t]
		fmt.Fprintf(&sb, "\n### %s\n\n", label)
		for _, c := range commits {
			writeCommitLine(&sb, c)
		}
	}

	// Render commits without a recognized type under "Other Changes".
	if other, ok := groups[""]; ok {
		fmt.Fprintf(&sb, "\n### Other Changes\n\n")
		for _, c := range other {
			writeCommitLine(&sb, c)
		}
	}

	return sb.String()
}

func writeCommitLine(sb *strings.Builder, c Commit) {
	prefix := ""
	if c.Scope != "" {
		prefix = fmt.Sprintf("**%s:** ", c.Scope)
	}
	breaking := ""
	if c.IsBreaking {
		breaking = " **BREAKING**"
	}
	fmt.Fprintf(sb, "- %s%s%s\n", prefix, c.Subject, breaking)
}
