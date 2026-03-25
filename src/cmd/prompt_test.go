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
		"open console":         {skill: "", usageFile: "usage_prompt.jsonl", title: "Consulting AI", detail: "Manual Prompting"},
		"/maggus-plan":         {skill: "/maggus-plan", usageFile: "usage_plan.jsonl", title: "Planning", detail: "Manual Prompting"},
		"/maggus-vision":       {skill: "/maggus-vision", usageFile: "usage_vision.jsonl", title: "Defining a vision", detail: "Manual Prompting"},
		"/maggus-architecture": {skill: "/maggus-architecture", usageFile: "usage_architecture.jsonl", title: "Architecture", detail: "Manual Prompting"},
		"/maggus-bugreport":    {skill: "/maggus-bugreport", usageFile: "usage_bugreport.jsonl", title: "Creating bug ticket", detail: "Manual Prompting"},
		"/bryan-plan":          {skill: "/bryan-plan", usageFile: "usage_bryan_plan.jsonl", title: "Planning with bryan", detail: "Manual Prompting"},
		"/bryan-bugreport":     {skill: "/bryan-bugreport", usageFile: "usage_bryan_bugreport.jsonl", title: "Reporting bug to bryan", detail: "Manual Prompting"},
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
