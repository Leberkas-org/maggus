package runtracker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Run holds metadata for a single maggus work invocation.
type Run struct {
	ID          string
	Dir         string
	Branch      string
	Model       string
	Iterations  int
	StartCommit string
	StartTime   time.Time
}

// New creates a Run, generates the RUN_ID, creates the run directory, and writes the initial run.md.
func New(workDir string, model string, iterations int) (*Run, error) {
	now := time.Now()
	id := now.Format("20060102-150405")
	dir := filepath.Join(workDir, ".maggus", "runs", id)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create run directory: %w", err)
	}

	branch := gitCurrentBranch(workDir)
	startCommit := gitHeadCommit(workDir)

	r := &Run{
		ID:          id,
		Dir:         dir,
		Branch:      branch,
		Model:       model,
		Iterations:  iterations,
		StartCommit: startCommit,
		StartTime:   now,
	}

	if err := r.writeStartFile(); err != nil {
		return nil, err
	}

	return r, nil
}

// IterationLogPath returns the path for the given iteration's log file (1-based, zero-padded).
func (r *Run) IterationLogPath(iteration int) string {
	return filepath.Join(r.Dir, fmt.Sprintf("iteration-%02d.md", iteration))
}

// Finalize appends end-of-run metadata to run.md.
func (r *Run) Finalize(workDir string) error {
	endTime := time.Now()
	endCommit := gitHeadCommit(workDir)

	commitRange := ""
	if r.StartCommit != "" && endCommit != "" && r.StartCommit != endCommit {
		commitRange = r.StartCommit[:minLen(r.StartCommit, 7)] + ".." + endCommit[:minLen(endCommit, 7)]
	}

	runMdPath := filepath.Join(r.Dir, "run.md")
	f, err := os.OpenFile(runMdPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open run.md for append: %w", err)
	}
	defer f.Close()

	var b strings.Builder
	b.WriteString("\n## End\n\n")
	fmt.Fprintf(&b, "- **End Time:** %s\n", endTime.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **End Commit:** %s\n", endCommit)
	fmt.Fprintf(&b, "- **Commit Range:** %s\n", commitRange)

	_, err = f.WriteString(b.String())
	return err
}

// PrintSummary prints a summary banner to stdout.
func (r *Run) PrintSummary(workDir string) {
	endCommit := gitHeadCommit(workDir)
	commitRange := ""
	if r.StartCommit != "" && endCommit != "" && r.StartCommit != endCommit {
		commitRange = r.StartCommit[:minLen(r.StartCommit, 7)] + ".." + endCommit[:minLen(endCommit, 7)]
	}

	fmt.Println()
	fmt.Println("══════════════════════════════════════════")
	fmt.Println("  Maggus Run Summary")
	fmt.Println("══════════════════════════════════════════")
	fmt.Printf("  RUN_ID:       %s\n", r.ID)
	fmt.Printf("  Branch:       %s\n", r.Branch)
	fmt.Printf("  Logs:         %s\n", r.Dir)
	fmt.Printf("  Commit Range: %s\n", commitRange)
	fmt.Println("══════════════════════════════════════════")
	fmt.Println()
}

func (r *Run) writeStartFile() error {
	runMdPath := filepath.Join(r.Dir, "run.md")

	var b strings.Builder
	b.WriteString("# Run Log\n\n")
	fmt.Fprintf(&b, "- **RUN_ID:** %s\n", r.ID)
	fmt.Fprintf(&b, "- **Branch:** %s\n", r.Branch)
	fmt.Fprintf(&b, "- **Model:** %s\n", r.Model)
	fmt.Fprintf(&b, "- **Iterations:** %d\n", r.Iterations)
	fmt.Fprintf(&b, "- **Start Commit:** %s\n", r.StartCommit)
	fmt.Fprintf(&b, "- **Start Time:** %s\n", r.StartTime.Format(time.RFC3339))

	return os.WriteFile(runMdPath, []byte(b.String()), 0o644)
}

func gitCurrentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "(unknown)"
	}
	return strings.TrimSpace(string(out))
}

func gitHeadCommit(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func minLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}
