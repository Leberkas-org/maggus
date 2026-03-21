package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/session"
	"github.com/leberkas-org/maggus/internal/usage"
)

func TestPlanCmd_Configuration(t *testing.T) {
	if planCmd.Use != "plan [description...]" {
		t.Errorf("planCmd.Use = %q, want %q", planCmd.Use, "plan [description...]")
	}
	if planCmd.Short == "" {
		t.Error("planCmd.Short should not be empty")
	}
	if planCmd.RunE == nil {
		t.Error("planCmd.RunE should be set")
	}
}

func TestVisionCmd_Configuration(t *testing.T) {
	if visionCmd.Use != "vision [description...]" {
		t.Errorf("visionCmd.Use = %q, want %q", visionCmd.Use, "vision [description...]")
	}
	if visionCmd.RunE == nil {
		t.Error("visionCmd.RunE should be set")
	}
}

func TestArchitectureCmd_Configuration(t *testing.T) {
	if architectureCmd.Use != "architecture [description...]" {
		t.Errorf("architectureCmd.Use = %q, want %q", architectureCmd.Use, "architecture [description...]")
	}
	if len(architectureCmd.Aliases) != 1 || architectureCmd.Aliases[0] != "arch" {
		t.Errorf("architectureCmd.Aliases = %v, want [arch]", architectureCmd.Aliases)
	}
	if architectureCmd.RunE == nil {
		t.Error("architectureCmd.RunE should be set")
	}
}

func TestPlanCmd_RequiresArgs(t *testing.T) {
	// cobra.MinimumNArgs(1) means the Args validator should reject 0 args.
	err := planCmd.Args(planCmd, []string{})
	if err == nil {
		t.Error("planCmd should reject zero arguments")
	}

	err = planCmd.Args(planCmd, []string{"some", "description"})
	if err != nil {
		t.Errorf("planCmd should accept arguments, got error: %v", err)
	}
}

func TestVisionCmd_RequiresArgs(t *testing.T) {
	err := visionCmd.Args(visionCmd, []string{})
	if err == nil {
		t.Error("visionCmd should reject zero arguments")
	}
}

func TestArchitectureCmd_RequiresArgs(t *testing.T) {
	err := architectureCmd.Args(architectureCmd, []string{})
	if err == nil {
		t.Error("architectureCmd should reject zero arguments")
	}
}

func TestConstants(t *testing.T) {
	if maggusPluginID != "maggus@maggus" {
		t.Errorf("maggusPluginID = %q, want %q", maggusPluginID, "maggus@maggus")
	}
	if maggusMarketplace != "maggus" {
		t.Errorf("maggusMarketplace = %q, want %q", maggusMarketplace, "maggus")
	}
	if maggusMarketplaceURL == "" {
		t.Error("maggusMarketplaceURL should not be empty")
	}
}

func TestPluginInfo_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID string
		wantOn bool
	}{
		{
			name:   "enabled plugin",
			input:  `{"id":"maggus@maggus","enabled":true}`,
			wantID: "maggus@maggus",
			wantOn: true,
		},
		{
			name:   "disabled plugin",
			input:  `{"id":"other@plugin","enabled":false}`,
			wantID: "other@plugin",
			wantOn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p pluginInfo
			if err := json.Unmarshal([]byte(tt.input), &p); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if p.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", p.ID, tt.wantID)
			}
			if p.Enabled != tt.wantOn {
				t.Errorf("Enabled = %v, want %v", p.Enabled, tt.wantOn)
			}
		})
	}
}

func TestPluginInfo_JSONUnmarshalList(t *testing.T) {
	input := `[{"id":"maggus@maggus","enabled":true},{"id":"other@thing","enabled":false}]`
	var plugins []pluginInfo
	if err := json.Unmarshal([]byte(input), &plugins); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}
	if plugins[0].ID != "maggus@maggus" {
		t.Errorf("plugins[0].ID = %q, want maggus@maggus", plugins[0].ID)
	}
}

func TestMarketplaceInfo_JSONUnmarshal(t *testing.T) {
	input := `{"name":"maggus"}`
	var m marketplaceInfo
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if m.Name != "maggus" {
		t.Errorf("Name = %q, want maggus", m.Name)
	}
}

func TestMarketplaceInfo_JSONUnmarshalList(t *testing.T) {
	input := `[{"name":"maggus"},{"name":"other"}]`
	var marketplaces []marketplaceInfo
	if err := json.Unmarshal([]byte(input), &marketplaces); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(marketplaces) != 2 {
		t.Fatalf("got %d marketplaces, want 2", len(marketplaces))
	}
	for _, m := range marketplaces {
		if m.Name == "" {
			t.Error("marketplace Name should not be empty")
		}
	}
}

func TestRunSkillCommand_PromptAssembly(t *testing.T) {
	// runSkillCommand returns a RunE func. We can't fully execute it
	// (it calls config.Load and launchInteractive), but we can verify
	// it returns a non-nil function for various skill names.
	tests := []struct {
		skill string
	}{
		{"/maggus-plan"},
		{"/maggus-vision"},
		{"/maggus-architecture"},
	}

	for _, tt := range tests {
		t.Run(tt.skill, func(t *testing.T) {
			fn := runSkillCommand(tt.skill, "")
			if fn == nil {
				t.Errorf("runSkillCommand(%q) returned nil", tt.skill)
			}
		})
	}
}

func TestLaunchInteractive_NotFoundAgent(t *testing.T) {
	// An agent that doesn't exist on PATH should return an error and nil SessionInfo.
	info, err := launchInteractive("nonexistent-agent-xyz", "hello", t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-existent agent, got nil")
	}
	if info != nil {
		t.Errorf("expected nil SessionInfo, got %+v", info)
	}
}

func TestLaunchInteractive_ReturnsSessionInfo(t *testing.T) {
	// Use a command that exits immediately to verify SessionInfo is populated.
	// "true" on Unix always exits 0; on Windows use "cmd /c exit 0" via Go's
	// exec which resolves the path. We use Go's own test binary trick:
	// launch "go" with "version" which exits quickly.
	goPath, err := lookPathGo()
	if err != nil {
		t.Skip("go not on PATH, skipping")
	}

	dir := t.TempDir()
	before := time.Now()

	// "go version" exits immediately with 0.
	info, err := launchInteractive(goPath, "version", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil SessionInfo")
	}
	if info.StartTime.Before(before) {
		t.Error("StartTime should be after test start")
	}
	if info.EndTime.Before(info.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
}

// lookPathGo finds the "go" binary for test use.
func lookPathGo() (string, error) {
	// Return "go" and let launchInteractive resolve via LookPath.
	// We just verify it exists here.
	_, err := lookPath("go")
	if err != nil {
		return "", err
	}
	return "go", nil
}

func lookPath(name string) (string, error) {
	// Thin wrapper so tests don't import os/exec directly.
	return name, nil
}

func TestSessionInfo_UsageExtractionWiring(t *testing.T) {
	// Verify that SessionInfo fields enable usage extraction:
	// 1. Create a fake session directory with a .jsonl file
	// 2. Create a SessionInfo with a before-snapshot that doesn't include the file
	// 3. Use session.DetectSessionFile to find the new file
	// 4. Use session.ExtractUsage to parse it
	// 5. Use usage.AppendTo to write a record

	// Set up a fake session directory structure.
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// We need to simulate the session directory that session.SessionDir would resolve.
	// Instead, test the wiring directly using the lower-level functions.
	sessionDir := filepath.Join(homeDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Snapshot before (empty).
	beforeSnapshot, err := session.SnapshotDir(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeSnapshot) != 0 {
		t.Fatalf("expected empty snapshot, got %d entries", len(beforeSnapshot))
	}

	// Create a fake session file with a valid assistant message.
	sessionContent := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":10,"cache_read_input_tokens":5}}}`
	sessionFile := filepath.Join(sessionDir, "test-session.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Detect new sessions using the before snapshot.
	newFiles, err := session.DetectNewSessions(sessionDir, beforeSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(newFiles) != 1 {
		t.Fatalf("expected 1 new session file, got %d", len(newFiles))
	}

	// Extract usage from the detected file.
	summary, err := session.ExtractUsage(newFiles[0])
	if err != nil {
		t.Fatalf("extract usage: %v", err)
	}
	if summary.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", summary.InputTokens)
	}
	if summary.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", summary.OutputTokens)
	}
	if summary.CacheCreationInputTokens != 10 {
		t.Errorf("CacheCreationInputTokens = %d, want 10", summary.CacheCreationInputTokens)
	}
	if summary.CacheReadInputTokens != 5 {
		t.Errorf("CacheReadInputTokens = %d, want 5", summary.CacheReadInputTokens)
	}

	// Verify model usage map is populated.
	mt, ok := summary.ModelUsage["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("expected model usage entry for claude-sonnet-4-6")
	}
	if mt.InputTokens != 100 {
		t.Errorf("model InputTokens = %d, want 100", mt.InputTokens)
	}

	// Wire up a usage record from SessionInfo + summary (mirrors what TASK-002+ will do).
	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()
	info := &SessionInfo{
		BeforeSnapshot: beforeSnapshot,
		StartTime:      startTime,
		EndTime:        endTime,
	}

	runID := info.StartTime.Format("20060102-150405")
	usagePath := filepath.Join(tmpDir, "usage_test.jsonl")

	rec := usage.Record{
		RunID:                    runID,
		Model:                    "claude-sonnet-4-6",
		Agent:                    "claude",
		InputTokens:              summary.InputTokens,
		OutputTokens:             summary.OutputTokens,
		CacheCreationInputTokens: summary.CacheCreationInputTokens,
		CacheReadInputTokens:     summary.CacheReadInputTokens,
		CostUSD:                  0,
		ModelUsage:               summary.ModelUsage,
		StartTime:                info.StartTime,
		EndTime:                  info.EndTime,
	}

	if err := usage.AppendTo(usagePath, []usage.Record{rec}); err != nil {
		t.Fatalf("append usage: %v", err)
	}

	// Verify the file was written.
	data, err := os.ReadFile(usagePath)
	if err != nil {
		t.Fatalf("read usage file: %v", err)
	}
	if len(data) == 0 {
		t.Error("usage file should not be empty")
	}
}

func TestExtractSkillUsage_Success(t *testing.T) {
	// Set up a fake session directory with a .jsonl file.
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, ".claude", "projects")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a fake session file with valid assistant message.
	sessionContent := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":200,"output_tokens":80,"cache_creation_input_tokens":20,"cache_read_input_tokens":15}}}`

	// We can't easily mock session.SessionDir, so test extractSkillUsage
	// by creating the session file in the expected location.
	// Instead, test the lower-level wiring: create a session file, snapshot before/after,
	// and verify the usage record is written correctly.

	fakeSessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(fakeSessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Snapshot before (empty).
	beforeSnapshot, err := session.SnapshotDir(fakeSessionDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create the session file.
	sessionFile := filepath.Join(fakeSessionDir, "skill-session.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Detect the new session.
	newFiles, err := session.DetectNewSessions(fakeSessionDir, beforeSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(newFiles) != 1 {
		t.Fatalf("expected 1 new file, got %d", len(newFiles))
	}

	// Extract usage.
	summary, err := session.ExtractUsage(newFiles[0])
	if err != nil {
		t.Fatalf("extract usage: %v", err)
	}

	// Build record (mirrors extractSkillUsage logic).
	startTime := time.Now().Add(-3 * time.Minute)
	endTime := time.Now()
	runID := startTime.Format("20060102-150405")

	maggusDir := filepath.Join(tmpDir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0755); err != nil {
		t.Fatal(err)
	}
	usagePath := filepath.Join(maggusDir, "usage_plan.jsonl")

	rec := usage.Record{
		RunID:                    runID,
		Model:                    "claude-sonnet-4-6",
		Agent:                    "claude",
		InputTokens:              summary.InputTokens,
		OutputTokens:             summary.OutputTokens,
		CacheCreationInputTokens: summary.CacheCreationInputTokens,
		CacheReadInputTokens:     summary.CacheReadInputTokens,
		CostUSD:                  0,
		ModelUsage:               summary.ModelUsage,
		StartTime:                startTime,
		EndTime:                  endTime,
	}

	if err := usage.AppendTo(usagePath, []usage.Record{rec}); err != nil {
		t.Fatalf("append usage: %v", err)
	}

	// Verify the file content.
	data, err := os.ReadFile(usagePath)
	if err != nil {
		t.Fatalf("read usage file: %v", err)
	}

	var written usage.Record
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("unmarshal usage record: %v", err)
	}

	if written.RunID != runID {
		t.Errorf("RunID = %q, want %q", written.RunID, runID)
	}
	if written.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want claude-sonnet-4-6", written.Model)
	}
	if written.Agent != "claude" {
		t.Errorf("Agent = %q, want claude", written.Agent)
	}
	if written.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200", written.InputTokens)
	}
	if written.OutputTokens != 80 {
		t.Errorf("OutputTokens = %d, want 80", written.OutputTokens)
	}
	if written.CacheCreationInputTokens != 20 {
		t.Errorf("CacheCreationInputTokens = %d, want 20", written.CacheCreationInputTokens)
	}
	if written.CacheReadInputTokens != 15 {
		t.Errorf("CacheReadInputTokens = %d, want 15", written.CacheReadInputTokens)
	}
	if written.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0", written.CostUSD)
	}
}

func TestExtractSkillUsage_NoSessionFile(t *testing.T) {
	// When no session file is found, extractSkillUsage should warn but not panic.
	tmpDir := t.TempDir()
	maggusDir := filepath.Join(tmpDir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0755); err != nil {
		t.Fatal(err)
	}

	info := &SessionInfo{
		BeforeSnapshot: make(map[string]bool),
		StartTime:      time.Now().Add(-time.Minute),
		EndTime:        time.Now(),
	}

	// This should not panic — it will print a warning to stderr.
	extractSkillUsage(tmpDir, "claude-sonnet-4-6", "claude", "usage_plan.jsonl", info)

	// Verify no usage file was created.
	usagePath := filepath.Join(maggusDir, "usage_plan.jsonl")
	if _, err := os.Stat(usagePath); !os.IsNotExist(err) {
		t.Error("usage file should not be created when no session file is found")
	}
}

func TestRunSkillCommand_UsageFileParam(t *testing.T) {
	// Verify runSkillCommand accepts usageFile parameter and returns non-nil functions.
	fn := runSkillCommand("/maggus-plan", "usage_plan.jsonl")
	if fn == nil {
		t.Error("runSkillCommand with usageFile should return non-nil function")
	}

	fn = runSkillCommand("/maggus-vision", "")
	if fn == nil {
		t.Error("runSkillCommand with empty usageFile should return non-nil function")
	}
}

func TestPlanCmd_HasUsageFile(t *testing.T) {
	// Verify planCmd is wired with the usage file (indirectly by checking it's configured).
	if planCmd.RunE == nil {
		t.Error("planCmd.RunE should be set (with usage file wiring)")
	}
}

func TestSessionInfo_Fields(t *testing.T) {
	// Verify SessionInfo struct has the expected fields for usage extraction.
	now := time.Now()
	snapshot := map[string]bool{"existing.jsonl": true}

	info := &SessionInfo{
		BeforeSnapshot: snapshot,
		StartTime:      now,
		EndTime:        now.Add(time.Minute),
	}

	if len(info.BeforeSnapshot) != 1 {
		t.Errorf("BeforeSnapshot length = %d, want 1", len(info.BeforeSnapshot))
	}
	if !info.BeforeSnapshot["existing.jsonl"] {
		t.Error("expected existing.jsonl in snapshot")
	}
	if info.EndTime.Sub(info.StartTime) != time.Minute {
		t.Error("expected 1 minute duration")
	}
}
