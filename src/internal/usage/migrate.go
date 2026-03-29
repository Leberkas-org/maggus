package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
)

// legacyRecord is the old per-project usage record format used before global tracking was added.
type legacyRecord struct {
	RunID                    string                       `json:"run_id"`
	TaskID                   string                       `json:"task_id"`
	TaskTitle                string                       `json:"task_title"`
	Model                    string                       `json:"model"`
	Agent                    string                       `json:"agent"`
	InputTokens              int                          `json:"input_tokens"`
	OutputTokens             int                          `json:"output_tokens"`
	CacheCreationInputTokens int                          `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int                          `json:"cache_read_input_tokens"`
	CostUSD                  float64                      `json:"cost_usd"`
	ModelUsage               map[string]agent.ModelTokens `json:"model_usage"`
	StartTime                time.Time                    `json:"start_time"`
	EndTime                  time.Time                    `json:"end_time"`
}

// sessionFileKinds maps old per-project session file names to their usage Kind.
var sessionFileKinds = map[string]string{
	"usage_plan.jsonl":            "plan",
	"usage_prompt.jsonl":          "prompt",
	"usage_bugreport.jsonl":       "bugreport",
	"usage_vision.jsonl":          "vision",
	"usage_architecture.jsonl":    "architecture",
	"usage_bryan_plan.jsonl":      "bryan_plan",
	"usage_bryan_bugreport.jsonl": "bryan_bugreport",
}

// legacyCSVFileNames are incompatible old CSV files that are renamed without migration.
var legacyCSVFileNames = []string{"usage.csv", "usage_v3.csv"}

// MigrateProject migrates per-project usage data from projectDir/.maggus/ to ~/.maggus/usage/.
// Files that do not exist are skipped silently. After successful migration each source file is
// renamed with a ".migrated" suffix so subsequent calls are no-ops (idempotent).
func MigrateProject(projectDir string) error {
	maggusDir := filepath.Join(projectDir, ".maggus")
	repoURL := gitutil.RepoURL(projectDir)

	globalDir, err := globalconfig.Dir()
	if err != nil {
		return fmt.Errorf("get global config dir: %w", err)
	}
	globalUsageDir := filepath.Join(globalDir, "usage")

	if err := migrateWorkFile(maggusDir, globalUsageDir, repoURL); err != nil {
		return err
	}

	for filename, kind := range sessionFileKinds {
		if err := migrateSessionFile(maggusDir, globalUsageDir, filename, kind, repoURL); err != nil {
			return err
		}
	}

	for _, name := range legacyCSVFileNames {
		if err := markMigrated(filepath.Join(maggusDir, name)); err != nil {
			return err
		}
	}

	return nil
}

// migrateWorkFile reads legacy work records and appends them to globalUsageDir/work.jsonl.
func migrateWorkFile(maggusDir, globalUsageDir, repoURL string) error {
	src := filepath.Join(maggusDir, "usage_work.jsonl")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	records, err := readLegacyRecords(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	if len(records) > 0 {
		if err := os.MkdirAll(globalUsageDir, 0o755); err != nil {
			return fmt.Errorf("create global usage dir: %w", err)
		}
		newRecords := make([]Record, len(records))
		for i, lr := range records {
			newRecords[i] = legacyToRecord(lr, "", repoURL)
		}
		if err := AppendTo(filepath.Join(globalUsageDir, "work.jsonl"), newRecords); err != nil {
			return fmt.Errorf("append work records: %w", err)
		}
	}

	return markMigrated(src)
}

// migrateSessionFile reads legacy session records and appends them to globalUsageDir/sessions.jsonl.
func migrateSessionFile(maggusDir, globalUsageDir, filename, kind, repoURL string) error {
	src := filepath.Join(maggusDir, filename)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	records, err := readLegacyRecords(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	if len(records) > 0 {
		if err := os.MkdirAll(globalUsageDir, 0o755); err != nil {
			return fmt.Errorf("create global usage dir: %w", err)
		}
		newRecords := make([]Record, len(records))
		for i, lr := range records {
			newRecords[i] = legacyToRecord(lr, kind, repoURL)
		}
		if err := AppendTo(filepath.Join(globalUsageDir, "sessions.jsonl"), newRecords); err != nil {
			return fmt.Errorf("append session records: %w", err)
		}
	}

	return markMigrated(src)
}

// readLegacyRecords reads all JSON lines from path as legacyRecord values.
// Malformed lines are silently skipped.
func readLegacyRecords(path string) ([]legacyRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []legacyRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var lr legacyRecord
		if err := json.Unmarshal([]byte(line), &lr); err != nil {
			continue
		}
		records = append(records, lr)
	}
	return records, scanner.Err()
}

// legacyToRecord converts a legacyRecord to a new Record with the given Kind and Repository.
func legacyToRecord(lr legacyRecord, kind, repoURL string) Record {
	return Record{
		RunID:                    lr.RunID,
		Repository:               repoURL,
		Kind:                     kind,
		TaskShort:                lr.TaskID,
		TaskTitle:                lr.TaskTitle,
		Model:                    lr.Model,
		Agent:                    lr.Agent,
		InputTokens:              lr.InputTokens,
		OutputTokens:             lr.OutputTokens,
		CacheCreationInputTokens: lr.CacheCreationInputTokens,
		CacheReadInputTokens:     lr.CacheReadInputTokens,
		CostUSD:                  lr.CostUSD,
		ModelUsage:               lr.ModelUsage,
		StartTime:                lr.StartTime,
		EndTime:                  lr.EndTime,
	}
}

// markMigrated renames path to path+".migrated". If path does not exist, this is a no-op.
func markMigrated(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if err := os.Rename(path, path+".migrated"); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}
