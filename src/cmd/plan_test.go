package cmd

import (
	"encoding/json"
	"testing"
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
			fn := runSkillCommand(tt.skill)
			if fn == nil {
				t.Errorf("runSkillCommand(%q) returned nil", tt.skill)
			}
		})
	}
}
