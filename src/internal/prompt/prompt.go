package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type BuildOptions struct {
	TaskContent    string
	FeatureContext string
	RepoPath       string
}

func Build(opts BuildOptions) string {
	var b strings.Builder

	if opts.FeatureContext != "" {
		b.WriteString(opts.FeatureContext)
		b.WriteString("\n\n---\n\n")
	}

	b.WriteString(opts.TaskContent)

	memory := readMemory(opts.RepoPath)
	if memory != "" {
		b.WriteString("\n\n---\n\n## Project Memory\n\n")
		b.WriteString(memory)
	}

	bootstrap := readBootstrap(opts.RepoPath)
	if bootstrap != "" {
		b.WriteString("\n\n---\n\n## Bootstrap\n\n")
		b.WriteString(bootstrap)
	}

	return b.String()
}

func readMemory(repoPath string) string {
	path := filepath.Join(repoPath, ".maggus", "MEMORY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readBootstrap(repoPath string) string {
	candidates := []string{"CLAUDE.md", "AGENTS.md"}
	var parts []string
	for _, name := range candidates {
		path := filepath.Join(repoPath, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			parts = append(parts, fmt.Sprintf("### %s\n\n%s", name, content))
		}
	}
	return strings.Join(parts, "\n\n")
}
