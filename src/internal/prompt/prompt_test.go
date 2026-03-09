package prompt

import (
	"strings"
	"testing"

	"github.com/dirnei/maggus/internal/parser"
)

func TestBuild(t *testing.T) {
	task := &parser.Task{
		ID:          "TASK-042",
		Title:       "Implement the thing",
		Description: "As a dev, I want the thing so it works.",
		Criteria: []parser.Criterion{
			{Text: "First criterion", Checked: false},
			{Text: "Second criterion", Checked: true},
		},
	}

	result := Build(task)

	checks := []string{
		"TASK-042",
		"Implement the thing",
		"As a dev, I want the thing so it works.",
		"- [ ] First criterion",
		"- [x] Second criterion",
		"Focus only on this task",
		"verify that every acceptance criterion",
	}

	for _, want := range checks {
		if !strings.Contains(result, want) {
			t.Errorf("prompt missing %q\n\nGot:\n%s", want, result)
		}
	}
}
