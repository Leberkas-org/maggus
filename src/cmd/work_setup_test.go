package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/spf13/cobra"
)

// fakeAgent implements agent.Agent for testing without requiring any CLI tool.
type fakeAgent struct {
	name        string
	validateErr error
}

func (f *fakeAgent) Run(_ context.Context, _ string, _ string, _ *tea.Program) error { return nil }
func (f *fakeAgent) RunOnce(_ context.Context, _ string, _ string) (string, error)   { return "", nil }
func (f *fakeAgent) Name() string                                                     { return f.name }
func (f *fakeAgent) Validate() error                                                  { return f.validateErr }

// stubSetupDeps replaces workSetup's function variables with test stubs
// and restores originals on cleanup.
type setupStubs struct {
	loadConfig    func(string) (config.Config, error)
	newAgent      func(string) (agent.Agent, error)
	ensureIgnore  func(string) ([]string, error)
	fingerprint   func() (string, error)
	getwd         func() (string, error)
}

func stubSetupDeps(t *testing.T, s setupStubs) {
	t.Helper()

	origLoad := loadConfigFn
	origAgent := newAgentFn
	origIgnore := ensureGitignoreFn
	origFP := fingerprintGetFn
	origGetwd := getwdFn

	if s.loadConfig != nil {
		loadConfigFn = s.loadConfig
	}
	if s.newAgent != nil {
		newAgentFn = s.newAgent
	}
	if s.ensureIgnore != nil {
		ensureGitignoreFn = s.ensureIgnore
	}
	if s.fingerprint != nil {
		fingerprintGetFn = s.fingerprint
	}
	if s.getwd != nil {
		getwdFn = s.getwd
	}

	t.Cleanup(func() {
		loadConfigFn = origLoad
		newAgentFn = origAgent
		ensureGitignoreFn = origIgnore
		fingerprintGetFn = origFP
		getwdFn = origGetwd
	})
}

// defaultSetupStubs returns stubs that simulate a minimal valid environment.
func defaultSetupStubs() setupStubs {
	return setupStubs{
		loadConfig: func(string) (config.Config, error) {
			return config.Config{Model: "sonnet"}, nil
		},
		newAgent: func(name string) (agent.Agent, error) {
			return &fakeAgent{name: name}, nil
		},
		ensureIgnore: func(string) ([]string, error) { return nil, nil },
		fingerprint:  func() (string, error) { return "test-fp", nil },
		getwd:        func() (string, error) { return "/fake/dir", nil },
	}
}

// saveAndRestoreFlags saves the package-level flag values and restores them on cleanup.
func saveAndRestoreFlags(t *testing.T) {
	t.Helper()
	origCount := countFlag
	origModel := modelFlag
	origAgent := agentFlag
	origTask := taskFlag
	origWorktree := worktreeFlag
	origNoWorktree := noWorktreeFlag
	t.Cleanup(func() {
		countFlag = origCount
		modelFlag = origModel
		agentFlag = origAgent
		taskFlag = origTask
		worktreeFlag = origWorktree
		noWorktreeFlag = origNoWorktree
	})
}

func newDummyCmd() *cobra.Command {
	return &cobra.Command{Use: "test"}
}

// --- Model resolution tests ---

func TestWorkSetup_ModelFlagOverridesConfig(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{Model: "haiku"}, nil
	}
	stubSetupDeps(t, stubs)

	modelFlag = "opus"
	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// opus should resolve to claude-opus-4-6
	if wc.resolvedModel != "claude-opus-4-6" {
		t.Errorf("resolvedModel = %q, want %q", wc.resolvedModel, "claude-opus-4-6")
	}
}

func TestWorkSetup_ConfigModelUsedWhenNoFlag(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{Model: "sonnet"}, nil
	}
	stubSetupDeps(t, stubs)

	modelFlag = ""
	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wc.resolvedModel != "claude-sonnet-4-6" {
		t.Errorf("resolvedModel = %q, want %q", wc.resolvedModel, "claude-sonnet-4-6")
	}
}

func TestWorkSetup_EmptyModelShowsDefault(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{Model: ""}, nil
	}
	stubSetupDeps(t, stubs)

	modelFlag = ""
	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wc.modelDisplay != "default" {
		t.Errorf("modelDisplay = %q, want %q", wc.modelDisplay, "default")
	}
}

// --- Agent resolution tests ---

func TestWorkSetup_AgentFlagOverridesConfig(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{Agent: "claude"}, nil
	}

	var receivedAgentName string
	stubs.newAgent = func(name string) (agent.Agent, error) {
		receivedAgentName = name
		return &fakeAgent{name: name}, nil
	}
	stubSetupDeps(t, stubs)

	agentFlag = "opencode"
	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAgentName != "opencode" {
		t.Errorf("agent created with name %q, want %q", receivedAgentName, "opencode")
	}
	if wc.activeAgent.Name() != "opencode" {
		t.Errorf("activeAgent.Name() = %q, want %q", wc.activeAgent.Name(), "opencode")
	}
}

func TestWorkSetup_ConfigAgentUsedWhenNoFlag(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{Agent: "opencode"}, nil
	}

	var receivedAgentName string
	stubs.newAgent = func(name string) (agent.Agent, error) {
		receivedAgentName = name
		return &fakeAgent{name: name}, nil
	}
	stubSetupDeps(t, stubs)

	agentFlag = ""
	countFlag = 1

	_, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAgentName != "opencode" {
		t.Errorf("agent created with name %q, want %q", receivedAgentName, "opencode")
	}
}

// --- Worktree flag precedence tests ---

func TestWorkSetup_WorktreeFlagOverridesConfig(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{Worktree: false}, nil
	}
	stubSetupDeps(t, stubs)

	worktreeFlag = true
	noWorktreeFlag = false
	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wc.useWorktree {
		t.Error("expected useWorktree=true when --worktree flag is set")
	}
}

func TestWorkSetup_NoWorktreeFlagOverridesAll(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{Worktree: true}, nil
	}
	stubSetupDeps(t, stubs)

	worktreeFlag = true
	noWorktreeFlag = true
	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wc.useWorktree {
		t.Error("expected useWorktree=false when --no-worktree flag is set (overrides --worktree and config)")
	}
}

func TestWorkSetup_ConfigWorktreeUsedWhenNoFlags(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{Worktree: true}, nil
	}
	stubSetupDeps(t, stubs)

	worktreeFlag = false
	noWorktreeFlag = false
	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wc.useWorktree {
		t.Error("expected useWorktree=true from config when no CLI flags set")
	}
}

// --- Count resolution tests ---

func TestWorkSetup_TaskFlagForcesCountOne(t *testing.T) {
	saveAndRestoreFlags(t)
	stubSetupDeps(t, defaultSetupStubs())

	taskFlag = "TASK-001"
	countFlag = 10

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wc.count != 1 {
		t.Errorf("count = %d, want 1 when --task flag is set", wc.count)
	}
}

func TestWorkSetup_ArgsOverrideCountFlag(t *testing.T) {
	saveAndRestoreFlags(t)
	stubSetupDeps(t, defaultSetupStubs())

	taskFlag = ""
	countFlag = 5

	wc, err := workSetup(newDummyCmd(), []string{"3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wc.count != 3 {
		t.Errorf("count = %d, want 3 from args", wc.count)
	}
}

func TestWorkSetup_InvalidCountArg(t *testing.T) {
	saveAndRestoreFlags(t)
	stubSetupDeps(t, defaultSetupStubs())

	taskFlag = ""
	countFlag = 5

	_, err := workSetup(newDummyCmd(), []string{"abc"})
	if err == nil {
		t.Fatal("expected error for invalid count arg")
	}
	if !strings.Contains(err.Error(), "invalid task count") {
		t.Errorf("error = %q, want it to contain 'invalid task count'", err.Error())
	}
}

func TestWorkSetup_ZeroCountArg(t *testing.T) {
	saveAndRestoreFlags(t)
	stubSetupDeps(t, defaultSetupStubs())

	taskFlag = ""

	_, err := workSetup(newDummyCmd(), []string{"0"})
	if err == nil {
		t.Fatal("expected error for zero count")
	}
	if !strings.Contains(err.Error(), "positive integer") {
		t.Errorf("error = %q, want it to contain 'positive integer'", err.Error())
	}
}

func TestWorkSetup_NegativeCountArg(t *testing.T) {
	saveAndRestoreFlags(t)
	stubSetupDeps(t, defaultSetupStubs())

	taskFlag = ""

	_, err := workSetup(newDummyCmd(), []string{"-1"})
	if err == nil {
		t.Fatal("expected error for negative count")
	}
	if !strings.Contains(err.Error(), "positive integer") {
		t.Errorf("error = %q, want it to contain 'positive integer'", err.Error())
	}
}

// --- Include validation tests ---

func TestWorkSetup_IncludeWarningsForMissingFiles(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.loadConfig = func(string) (config.Config, error) {
		return config.Config{
			Include: []string{"exists.md", "missing.md"},
		}, nil
	}
	// getwd returns a temp dir so ValidateIncludes can check real paths
	dir := t.TempDir()
	stubs.getwd = func() (string, error) { return dir, nil }
	stubSetupDeps(t, stubs)

	countFlag = 1
	taskFlag = ""

	// Create only exists.md
	if err := writeTestFile(t, dir, "exists.md", "content"); err != nil {
		t.Fatal(err)
	}

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(wc.includeWarnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(wc.includeWarnings), wc.includeWarnings)
	}
	if !strings.Contains(wc.includeWarnings[0], "missing.md") {
		t.Errorf("warning = %q, want it to mention missing.md", wc.includeWarnings[0])
	}
	if len(wc.validIncludes) != 1 || wc.validIncludes[0] != "exists.md" {
		t.Errorf("validIncludes = %v, want [exists.md]", wc.validIncludes)
	}
}

// --- Fingerprint tests ---

func TestWorkSetup_EmptyFingerprint_SetsUnknown(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.fingerprint = func() (string, error) { return "", nil }
	stubSetupDeps(t, stubs)

	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wc.hostFingerprint != "unknown" {
		t.Errorf("hostFingerprint = %q, want %q", wc.hostFingerprint, "unknown")
	}
}

func TestWorkSetup_Fingerprint_UsesReturnedValue(t *testing.T) {
	saveAndRestoreFlags(t)
	stubs := defaultSetupStubs()
	stubs.fingerprint = func() (string, error) { return "abc123", nil }
	stubSetupDeps(t, stubs)

	countFlag = 1

	wc, err := workSetup(newDummyCmd(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wc.hostFingerprint != "abc123" {
		t.Errorf("hostFingerprint = %q, want %q", wc.hostFingerprint, "abc123")
	}
}

// writeTestFile is a test helper that creates a file with the given content.
func writeTestFile(t *testing.T, dir, name, content string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
}
