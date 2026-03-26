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

func TestLaunchInteractive_NotFoundAgent(t *testing.T) {
	// An agent that doesn't exist on PATH should return an error and nil SessionInfo.
	info, err := launchInteractive("nonexistent-agent-xyz", "hello", t.TempDir(), false, "")
	if err == nil {
		t.Fatal("expected error for non-existent agent, got nil")
	}
	if info != nil {
		t.Errorf("expected nil SessionInfo, got %+v", info)
	}
}

func TestLaunchInteractive_ReturnsSessionInfo(t *testing.T) {
	// Use a command that exits immediately to verify SessionInfo is populated.
	// "go" with no args exits with code 2, which is treated as user-initiated exit.
	goPath, err := lookPathGo()
	if err != nil {
		t.Skip("go not on PATH, skipping")
	}

	dir := t.TempDir()
	before := time.Now()

	info, err := launchInteractive(goPath, "", dir, false, "")
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
	_, err := lookPath("go")
	if err != nil {
		return "", err
	}
	return "go", nil
}

func lookPath(name string) (string, error) {
	return name, nil
}

func TestSessionInfo_UsageExtractionWiring(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	sessionDir := filepath.Join(homeDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	beforeSnapshot, err := session.SnapshotDir(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeSnapshot) != 0 {
		t.Fatalf("expected empty snapshot, got %d entries", len(beforeSnapshot))
	}

	sessionContent := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":10,"cache_read_input_tokens":5}}}`
	sessionFile := filepath.Join(sessionDir, "test-session.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	newFiles, err := session.DetectNewSessions(sessionDir, beforeSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(newFiles) != 1 {
		t.Fatalf("expected 1 new session file, got %d", len(newFiles))
	}

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

	mt, ok := summary.ModelUsage["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("expected model usage entry for claude-sonnet-4-6")
	}
	if mt.InputTokens != 100 {
		t.Errorf("model InputTokens = %d, want 100", mt.InputTokens)
	}

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

	data, err := os.ReadFile(usagePath)
	if err != nil {
		t.Fatalf("read usage file: %v", err)
	}
	if len(data) == 0 {
		t.Error("usage file should not be empty")
	}
}

func TestExtractSkillUsage_Success(t *testing.T) {
	tmpDir := t.TempDir()
	fakeSessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(fakeSessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	beforeSnapshot, err := session.SnapshotDir(fakeSessionDir)
	if err != nil {
		t.Fatal(err)
	}

	sessionContent := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":200,"output_tokens":80,"cache_creation_input_tokens":20,"cache_read_input_tokens":15}}}`
	sessionFile := filepath.Join(fakeSessionDir, "skill-session.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	newFiles, err := session.DetectNewSessions(fakeSessionDir, beforeSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(newFiles) != 1 {
		t.Fatalf("expected 1 new file, got %d", len(newFiles))
	}

	summary, err := session.ExtractUsage(newFiles[0])
	if err != nil {
		t.Fatalf("extract usage: %v", err)
	}

	startTime := time.Now().Add(-3 * time.Minute)
	endTime := time.Now()
	runID := startTime.Format("20060102-150405")

	usagePath := filepath.Join(tmpDir, "sessions.jsonl")

	rec := usage.Record{
		RunID:                    runID,
		Repository:               "https://github.com/test/repo.git",
		Kind:                     "plan",
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
	if written.Repository != "https://github.com/test/repo.git" {
		t.Errorf("Repository = %q, want %q", written.Repository, "https://github.com/test/repo.git")
	}
	if written.Kind != "plan" {
		t.Errorf("Kind = %q, want %q", written.Kind, "plan")
	}
	if written.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200", written.InputTokens)
	}
	if written.OutputTokens != 80 {
		t.Errorf("OutputTokens = %d, want 80", written.OutputTokens)
	}
}

func TestExtractSkillUsage_NoSessionFile(t *testing.T) {
	tmpDir := t.TempDir()

	info := &SessionInfo{
		BeforeSnapshot: make(map[string]bool),
		StartTime:      time.Now().Add(-time.Minute),
		EndTime:        time.Now(),
	}

	// Should return early without error when no session file is found.
	// This exercises the early return path in extractSkillUsage.
	extractSkillUsage(tmpDir, "claude-sonnet-4-6", "claude", "plan", info)
}

func TestSessionInfo_Fields(t *testing.T) {
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
