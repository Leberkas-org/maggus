package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Importer struct {
	tasksDir string
}

func NewImporter(tasksDir string) *Importer {
	return &Importer{tasksDir: tasksDir}
}

type ItemMeta struct {
	ItemID      string     `yaml:"item_id"`
	Title       string     `yaml:"title"`
	RepoURL     string     `yaml:"repo_url"`
	Status      string     `yaml:"status"`
	Priority    int        `yaml:"priority"`
	AutoApprove bool       `yaml:"auto_approve"`
	CreatedAt   time.Time  `yaml:"created_at"`
	SourceFile  string     `yaml:"source_file"`
	Tasks       []TaskMeta `yaml:"tasks"`
}

type TaskMeta struct {
	ID     string `yaml:"id"`
	Title  string `yaml:"title"`
	Status string `yaml:"status"`
	File   string `yaml:"file"`
}

func (imp *Importer) Import(planPath string, repoURL string, autoApprove bool) (*WorkItem, error) {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}

	plan, err := Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	itemID := uuid.New().String()
	itemDir := filepath.Join(imp.tasksDir, itemID)

	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		return nil, fmt.Errorf("create item dir: %w", err)
	}

	// Write context.md
	if err := os.WriteFile(filepath.Join(itemDir, "context.md"), []byte(plan.Context), 0o644); err != nil {
		return nil, fmt.Errorf("write context: %w", err)
	}

	// Write task files and build metadata
	status := "pending"
	if autoApprove {
		status = "ready"
	}

	meta := ItemMeta{
		ItemID:      itemID,
		Title:       plan.Title,
		RepoURL:     repoURL,
		Status:      status,
		AutoApprove: autoApprove,
		CreatedAt:   time.Now(),
		SourceFile:  filepath.Base(planPath),
	}

	var taskSpecs []TaskSpec
	for _, t := range plan.Tasks {
		taskFile := fmt.Sprintf("task-%03d.md", t.Number)
		if err := os.WriteFile(filepath.Join(itemDir, taskFile), []byte(t.Content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", taskFile, err)
		}
		taskID := fmt.Sprintf("task-%03d", t.Number)
		meta.Tasks = append(meta.Tasks, TaskMeta{
			ID:     taskID,
			Title:  t.Title,
			Status: "pending",
			File:   taskFile,
		})
		taskSpecs = append(taskSpecs, TaskSpec{
			ID:      taskID,
			Title:   t.Title,
			Content: t.Content,
		})
	}

	// Write item.yaml
	yamlData, err := yaml.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal item.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "item.yaml"), yamlData, 0o644); err != nil {
		return nil, fmt.Errorf("write item.yaml: %w", err)
	}

	// Move original to source.md
	if err := os.Rename(planPath, filepath.Join(itemDir, "source.md")); err != nil {
		return nil, fmt.Errorf("move source: %w", err)
	}

	return &WorkItem{
		ID:          itemID,
		PlanFile:    filepath.Join(itemDir, "source.md"),
		RepoURL:     repoURL,
		Title:       plan.Title,
		Description: plan.Context,
		Tasks:       taskSpecs,
		Status:      ItemStatus(status),
	}, nil
}
