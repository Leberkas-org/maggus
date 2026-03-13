package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temporary git repo and returns its path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	// Create initial commit
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0644)
	run("add", ".")
	run("commit", "-m", "initial commit")
	return dir
}

func addCommit(t *testing.T, dir, filename, message string) {
	t.Helper()
	os.WriteFile(filepath.Join(dir, filename), []byte(message), 0644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit failed: %s\n%s", err, out)
	}
}

func addTag(t *testing.T, dir, tag string) {
	t.Helper()
	cmd := exec.Command("git", "tag", tag)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tag failed: %s\n%s", err, out)
	}
}

// --- FindLastTag tests ---

func TestFindLastTag_NoTags(t *testing.T) {
	dir := initTestRepo(t)
	tag, err := FindLastTag(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "" {
		t.Errorf("expected empty tag, got %q", tag)
	}
}

func TestFindLastTag_WithTag(t *testing.T) {
	dir := initTestRepo(t)
	addTag(t, dir, "v1.0.0")
	addCommit(t, dir, "a.txt", "feat: new thing")

	tag, err := FindLastTag(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %q", tag)
	}
}

func TestFindLastTag_MultipleTags(t *testing.T) {
	dir := initTestRepo(t)
	addTag(t, dir, "v1.0.0")
	addCommit(t, dir, "a.txt", "feat: a")
	addTag(t, dir, "v2.0.0")
	addCommit(t, dir, "b.txt", "feat: b")

	tag, err := FindLastTag(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", tag)
	}
}

// --- CommitsSinceTag tests ---

func TestCommitsSinceTag_NoTag(t *testing.T) {
	dir := initTestRepo(t)
	addCommit(t, dir, "a.txt", "feat: first feature")

	commits, err := CommitsSinceTag(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should include all commits (initial + feat)
	if len(commits) < 2 {
		t.Fatalf("expected at least 2 commits, got %d", len(commits))
	}
}

func TestCommitsSinceTag_WithTag(t *testing.T) {
	dir := initTestRepo(t)
	addTag(t, dir, "v1.0.0")
	addCommit(t, dir, "a.txt", "feat: new feature")
	addCommit(t, dir, "b.txt", "fix: a bug")

	commits, err := CommitsSinceTag(dir, "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
}

func TestCommitsSinceTag_NoCommitsSinceTag(t *testing.T) {
	dir := initTestRepo(t)
	addTag(t, dir, "v1.0.0")

	commits, err := CommitsSinceTag(dir, "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 commits, got %d", len(commits))
	}
}

// --- Conventional commit parsing tests ---

func TestParseConventional_Simple(t *testing.T) {
	c := Commit{Subject: "feat: add login"}
	parseConventional(&c)
	if c.Type != "feat" {
		t.Errorf("expected type feat, got %q", c.Type)
	}
	if c.Scope != "" {
		t.Errorf("expected empty scope, got %q", c.Scope)
	}
	if c.IsBreaking {
		t.Error("expected not breaking")
	}
}

func TestParseConventional_WithScope(t *testing.T) {
	c := Commit{Subject: "fix(auth): handle expired tokens"}
	parseConventional(&c)
	if c.Type != "fix" {
		t.Errorf("expected type fix, got %q", c.Type)
	}
	if c.Scope != "auth" {
		t.Errorf("expected scope auth, got %q", c.Scope)
	}
}

func TestParseConventional_BreakingBang(t *testing.T) {
	c := Commit{Subject: "feat!: remove deprecated API"}
	parseConventional(&c)
	if c.Type != "feat" {
		t.Errorf("expected type feat, got %q", c.Type)
	}
	if !c.IsBreaking {
		t.Error("expected breaking change from ! suffix")
	}
}

func TestParseConventional_BreakingScopeAndBang(t *testing.T) {
	c := Commit{Subject: "refactor(core)!: restructure modules"}
	parseConventional(&c)
	if c.Type != "refactor" {
		t.Errorf("expected type refactor, got %q", c.Type)
	}
	if c.Scope != "core" {
		t.Errorf("expected scope core, got %q", c.Scope)
	}
	if !c.IsBreaking {
		t.Error("expected breaking")
	}
}

func TestParseConventional_BreakingInBody(t *testing.T) {
	c := Commit{
		Subject: "feat: change config format",
		Body:    "BREAKING CHANGE: config.yml schema has changed",
	}
	parseConventional(&c)
	if !c.IsBreaking {
		t.Error("expected breaking change from body")
	}
}

func TestParseConventional_NonConventional(t *testing.T) {
	c := Commit{Subject: "update readme"}
	parseConventional(&c)
	if c.Type != "" {
		t.Errorf("expected empty type, got %q", c.Type)
	}
}

// --- GroupByType tests ---

func TestGroupByType(t *testing.T) {
	commits := []Commit{
		{Subject: "feat: a", Type: "feat"},
		{Subject: "feat: b", Type: "feat"},
		{Subject: "fix: c", Type: "fix"},
		{Subject: "update readme", Type: ""},
	}
	groups := GroupByType(commits)
	if len(groups["feat"]) != 2 {
		t.Errorf("expected 2 feat commits, got %d", len(groups["feat"]))
	}
	if len(groups["fix"]) != 1 {
		t.Errorf("expected 1 fix commit, got %d", len(groups["fix"]))
	}
	if len(groups[""]) != 1 {
		t.Errorf("expected 1 other commit, got %d", len(groups[""]))
	}
}

// --- FormatChangelog tests ---

func TestFormatChangelog_WithTag(t *testing.T) {
	groups := map[string][]Commit{
		"feat": {
			{Subject: "feat: add login", Type: "feat"},
			{Subject: "feat(ui): dark mode", Type: "feat", Scope: "ui"},
		},
		"fix": {
			{Subject: "fix: null pointer", Type: "fix"},
		},
	}
	result := FormatChangelog(groups, "v1.2.0")
	if !strings.Contains(result, "## v1.2.0") {
		t.Error("expected tag in heading")
	}
	if !strings.Contains(result, "### Features") {
		t.Error("expected Features section")
	}
	if !strings.Contains(result, "### Bug Fixes") {
		t.Error("expected Bug Fixes section")
	}
	if !strings.Contains(result, "**ui:**") {
		t.Error("expected scope prefix")
	}
}

func TestFormatChangelog_NoTag(t *testing.T) {
	groups := map[string][]Commit{
		"feat": {{Subject: "feat: something", Type: "feat"}},
	}
	result := FormatChangelog(groups, "")
	if !strings.Contains(result, "## Unreleased") {
		t.Error("expected Unreleased heading")
	}
}

func TestFormatChangelog_OtherChanges(t *testing.T) {
	groups := map[string][]Commit{
		"": {{Subject: "update docs", Type: ""}},
	}
	result := FormatChangelog(groups, "v1.0.0")
	if !strings.Contains(result, "### Other Changes") {
		t.Error("expected Other Changes section")
	}
	if !strings.Contains(result, "- update docs") {
		t.Error("expected commit line")
	}
}

func TestFormatChangelog_BreakingMarker(t *testing.T) {
	groups := map[string][]Commit{
		"feat": {{Subject: "feat!: remove old API", Type: "feat", IsBreaking: true}},
	}
	result := FormatChangelog(groups, "v2.0.0")
	if !strings.Contains(result, "**BREAKING**") {
		t.Error("expected BREAKING marker")
	}
}

// --- Integration test: full pipeline with real git repo ---

func TestIntegration_FullPipeline(t *testing.T) {
	dir := initTestRepo(t)
	addTag(t, dir, "v1.0.0")
	addCommit(t, dir, "a.txt", "feat: add user profiles")
	addCommit(t, dir, "b.txt", "fix(auth): token refresh")
	addCommit(t, dir, "c.txt", "update changelog")

	tag, err := FindLastTag(dir)
	if err != nil {
		t.Fatalf("FindLastTag: %v", err)
	}
	if tag != "v1.0.0" {
		t.Fatalf("expected v1.0.0, got %q", tag)
	}

	commits, err := CommitsSinceTag(dir, tag)
	if err != nil {
		t.Fatalf("CommitsSinceTag: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commits))
	}

	groups := GroupByType(commits)
	if len(groups["feat"]) != 1 {
		t.Errorf("expected 1 feat, got %d", len(groups["feat"]))
	}
	if len(groups["fix"]) != 1 {
		t.Errorf("expected 1 fix, got %d", len(groups["fix"]))
	}
	if len(groups[""]) != 1 {
		t.Errorf("expected 1 other, got %d", len(groups[""]))
	}

	changelog := FormatChangelog(groups, tag)
	if !strings.Contains(changelog, "### Features") {
		t.Error("missing Features section")
	}
	if !strings.Contains(changelog, "### Bug Fixes") {
		t.Error("missing Bug Fixes section")
	}
	if !strings.Contains(changelog, "### Other Changes") {
		t.Error("missing Other Changes section")
	}
	if !strings.Contains(changelog, "**auth:**") {
		t.Error("missing scope prefix")
	}
}
