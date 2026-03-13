// Package usage appends per-task token usage records to .maggus/usage.csv.
package usage

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const fileName = ".maggus/usage.csv"

// Record represents a single task usage entry.
type Record struct {
	RunID        string
	TaskID       string
	TaskTitle    string
	PlanFile     string
	Model        string
	Agent        string
	InputTokens  int
	OutputTokens int
	StartTime    time.Time
	EndTime      time.Time
}

// Append writes one or more usage records to .maggus/usage.csv, creating
// the file with a header row if it does not exist.
func Append(dir string, records []Record) error {
	if len(records) == 0 {
		return nil
	}

	path := filepath.Join(dir, fileName)

	// Check if file exists to decide whether to write the header.
	writeHeader := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		writeHeader = true
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open usage file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if writeHeader {
		if err := w.Write(header()); err != nil {
			return fmt.Errorf("write usage header: %w", err)
		}
	}

	for _, r := range records {
		elapsed := r.EndTime.Sub(r.StartTime).Truncate(time.Second)
		row := []string{
			r.RunID,
			r.TaskID,
			r.TaskTitle,
			r.PlanFile,
			r.Model,
			r.Agent,
			fmt.Sprintf("%d", r.InputTokens),
			fmt.Sprintf("%d", r.OutputTokens),
			r.StartTime.Format(time.RFC3339),
			r.EndTime.Format(time.RFC3339),
			elapsed.String(),
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write usage row: %w", err)
		}
	}

	return nil
}

func header() []string {
	return []string{
		"run_id",
		"task_id",
		"task_title",
		"plan_file",
		"model",
		"agent",
		"input_tokens",
		"output_tokens",
		"start_time",
		"end_time",
		"elapsed",
	}
}
