package cmd

import "testing"

func TestSkillVerbMappingCoversAllSkillMappings(t *testing.T) {
	// Every entry in skillMappings must have a corresponding verb in skillVerbMapping.
	for label := range skillMappings {
		verb, ok := skillVerbMapping[label]
		if !ok {
			t.Errorf("skillMappings entry %q has no corresponding skillVerbMapping entry", label)
			continue
		}
		if verb == "" {
			t.Errorf("skillVerbMapping[%q] is empty", label)
		}
	}
}

func TestSkillVerbMappingValues(t *testing.T) {
	// Verify the specific verb values for each skill.
	expected := map[string]string{
		"open console":         "Consulting",
		"/maggus-plan":         "Planning",
		"/maggus-vision":       "Visioning",
		"/maggus-architecture": "Architecting",
		"/maggus-bugreport":    "Reporting Bug",
		"/bryan-plan":          "Planning",
		"/bryan-bugreport":     "Reporting Bug",
	}

	for label, wantVerb := range expected {
		got, ok := skillVerbMapping[label]
		if !ok {
			t.Errorf("skillVerbMapping missing entry for %q", label)
			continue
		}
		if got != wantVerb {
			t.Errorf("skillVerbMapping[%q] = %q, want %q", label, got, wantVerb)
		}
	}

	// Ensure no extra entries exist in skillVerbMapping beyond what we expect.
	for label := range skillVerbMapping {
		if _, ok := expected[label]; !ok {
			t.Errorf("unexpected skillVerbMapping entry: %q", label)
		}
	}
}
