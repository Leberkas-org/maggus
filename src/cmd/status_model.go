package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/filewatcher"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

const (
	statusHeaderLines = 11 // title + daemon line + blank + tab bar (~2) + separator + blank + progress + blank + tasks header + separator
	progressBarWidth  = 10
)

var statusSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Lipgloss styles for the status command.
var (
	statusGreenStyle = lipgloss.NewStyle().Foreground(styles.Success)
	statusCyanStyle  = lipgloss.NewStyle().Foreground(styles.Primary)
	statusRedStyle   = lipgloss.NewStyle().Foreground(styles.Error)
	statusDimStyle   = lipgloss.NewStyle().Faint(true)
	statusDimGreen   = lipgloss.NewStyle().Faint(true).Foreground(styles.Success)
	statusBoldStyle  = lipgloss.NewStyle().Bold(true)
	statusBlueStyle  = lipgloss.NewStyle().Foreground(styles.Accent)
)

// statusModel is the bubbletea model for the interactive status TUI.
type statusModel struct {
	taskListComponent

	features     []parser.Plan
	showAll      bool
	nextTaskID   string
	nextTaskFile string
	agentName    string

	// Feature tab selection
	selectedFeature int // index into visibleFeatures()

	dir       string             // working directory for file operations
	approvals approval.Approvals // cached approvals; reloaded on reloadFeatures

	approvalRequired bool // from config; used when reloading features

	is2x bool // true when Claude is in 2x mode (border turns yellow)

	// Temporary status note (e.g. "feature approved")
	statusNote string

	// Feature-level delete confirmation
	confirmDeleteFeature bool
	deleteFeatureErr     string

	// Live log panel
	showLog       bool
	logLines      []string
	logScroll     int
	logAutoScroll bool
	daemon        daemonStatus

	// Rich live view from state.json
	snapshot     *runlog.StateSnapshot // nil when no snapshot available
	spinnerFrame int

	// Discord Rich Presence (nil when not configured)
	presence *discord.Presence

	// File watcher for live feature reload
	watcher   *filewatcher.Watcher
	watcherCh <-chan bool
}

func newStatusModel(features []parser.Plan, showAll bool, nextTaskID, nextTaskFile, agentName, dir string, showLog bool, approvalRequired bool) statusModel {
	m := statusModel{
		taskListComponent: taskListComponent{
			HeaderLines: statusHeaderLines,
		},
		features:         features,
		showAll:          showAll,
		nextTaskID:       nextTaskID,
		nextTaskFile:     nextTaskFile,
		agentName:        agentName,
		dir:              dir,
		showLog:          showLog,
		approvalRequired: approvalRequired,
		logAutoScroll:    true,
	}
	visible := m.visibleFeatures()
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(visible[0], showAll)
	}
	return m
}

// visibleFeatures returns the features that should be shown based on the showAll flag.
func (m statusModel) visibleFeatures() []parser.Plan {
	var visible []parser.Plan
	for _, f := range m.features {
		if f.Completed && !m.showAll {
			continue
		}
		visible = append(visible, f)
	}
	return visible
}

// rebuildForSelectedFeature rebuilds the selectable tasks and resets the cursor
// for the currently selected feature.
func (m *statusModel) rebuildForSelectedFeature() {
	visible := m.visibleFeatures()
	if m.selectedFeature >= len(visible) {
		m.selectedFeature = 0
	}
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(visible[m.selectedFeature], m.showAll)
	} else {
		m.Tasks = nil
	}
	m.Cursor = 0
	m.ScrollOffset = 0
}

// reloadFeatures reloads all features, bugs, and approvals from disk and rebuilds the current view.
func (m *statusModel) reloadFeatures() {
	plans, a, err := loadPlansWithApprovals(m.dir, true)
	if err != nil {
		m.rebuildForSelectedFeature()
		return
	}
	m.approvals = a
	m.features = plans
	pruneStaleApprovals(m.dir, plans)
	m.nextTaskID, m.nextTaskFile = findNextTask(plans)
	m.rebuildForSelectedFeature()
}

// syncDetailSuffix updates the component's DetailSuffix from statusNote.
func (m *statusModel) syncDetailSuffix() {
	if m.statusNote != "" {
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		m.DetailSuffix = "\n" + mutedStyle.Render("  "+m.statusNote)
	} else {
		m.DetailSuffix = ""
	}
}

// featureFilePath returns the full path to a plan's source file.
func (m statusModel) featureFilePath(p parser.Plan) string {
	return p.File
}

// buildProgressBar renders a colored progress bar.
func buildProgressBar(done, total int) string {
	return styles.ProgressBar(done, total, progressBarWidth)
}

// buildProgressBarPlain renders a plain ASCII progress bar.
func buildProgressBarPlain(done, total int) string {
	return styles.ProgressBarPlain(done, total, progressBarWidth)
}

// buildSelectableTasksForFeature returns the flat list of tasks for a single plan.
// When showAll is false, completed tasks are excluded.
func buildSelectableTasksForFeature(plan parser.Plan, showAll bool) []parser.Task {
	var selectable []parser.Task
	for _, t := range plan.Tasks {
		if !showAll && t.IsComplete() {
			continue
		}
		selectable = append(selectable, t)
	}
	return selectable
}

// loadPlansWithApprovals loads all plans and the current approval map.
func loadPlansWithApprovals(dir string, includeCompleted bool) ([]parser.Plan, approval.Approvals, error) {
	plans, err := parser.LoadPlans(dir, includeCompleted)
	if err != nil {
		return nil, nil, err
	}
	a, err := approval.Load(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("load approvals: %w", err)
	}
	return plans, a, nil
}

// isPlanApproved checks whether a plan is approved given the approval map and mode.
func isPlanApproved(p parser.Plan, a approval.Approvals, approvalRequired bool) bool {
	return approval.IsApproved(a, p.ApprovalKey(), approvalRequired)
}

// findNextTask returns the ID and source file of the next workable task across all plans.
// Bugs are prioritized over features.
func findNextTask(plans []parser.Plan) (string, string) {
	// Bugs first, then features
	for _, p := range plans {
		if p.Completed || !p.IsBug {
			continue
		}
		next := parser.FindNextIncomplete(p.Tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	for _, p := range plans {
		if p.Completed || p.IsBug {
			continue
		}
		next := parser.FindNextIncomplete(p.Tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	return "", ""
}

// pruneStaleApprovals collects all known approval keys from the combined plan
// list and calls approval.Prune to remove stale entries.
func pruneStaleApprovals(dir string, all []parser.Plan) {
	var knownIDs []string
	for i := range all {
		knownIDs = append(knownIDs, all[i].ApprovalKey())
	}
	_ = approval.Prune(dir, knownIDs)
}
