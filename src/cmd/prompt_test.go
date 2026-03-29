package cmd

import "testing"

func TestSkillVerbMappingCoversAllSkillMappings(t *testing.T) {
	// Every entry in skillMappings must have a corresponding verb in skillVerbMapping.
	for label := range skillMappings {
		verb, ok := skillMappings[label]
		if !ok {
			t.Errorf("skillMappings entry %q has no corresponding skillVerbMapping entry", label)
			continue
		}
		if verb.title == "" || verb.detail == "" {
			t.Errorf("skillVerbMapping[%q] is empty", label)
		}
	}
}

func TestSkillVerbMappingValues(t *testing.T) {
	// Verify the specific verb values for each skill.
	expected := map[string]skillMapping{
		"open console":         {skill: "", kind: "prompt", title: "Consulting AI", detail: "Manual Prompting"},
		"/maggus-plan":         {skill: "/maggus-plan", kind: "plan", title: "Planning", detail: "Manual Prompting"},
		"/maggus-vision":       {skill: "/maggus-vision", kind: "vision", title: "Defining a vision", detail: "Manual Prompting"},
		"/maggus-architecture": {skill: "/maggus-architecture", kind: "architecture", title: "Architecture", detail: "Manual Prompting"},
		"/maggus-bugreport":    {skill: "/maggus-bugreport", kind: "bugreport", title: "Creating bug ticket", detail: "Manual Prompting"},
		"/bryan-plan":          {skill: "/bryan-plan", kind: "bryan_plan", title: "Planning with bryan", detail: "Manual Prompting"},
		"/bryan-bugreport":     {skill: "/bryan-bugreport", kind: "bryan_bugreport", title: "Reporting bug to bryan", detail: "Manual Prompting"},
	}

	for label, wantVerb := range expected {
		got, ok := skillMappings[label]
		if !ok {
			t.Errorf("skillVerbMapping missing entry for %q", label)
			continue
		}
		if got != wantVerb {
			t.Errorf("skillVerbMapping[%q] = %q, want %q", label, got, wantVerb)
		}
	}

	// Ensure no extra entries exist in skillVerbMapping beyond what we expect.
	for label := range skillMappings {
		if _, ok := expected[label]; !ok {
			t.Errorf("unexpected skillVerbMapping entry: %q", label)
		}
	}
}
