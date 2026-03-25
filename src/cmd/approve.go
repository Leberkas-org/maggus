package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/spf13/cobra"
)

// featureIDFromPath extracts the feature ID (base filename without extension) from a path.
// For example: ".maggus/features/feature_003.md" → "feature_003"
func featureIDFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".md")
	base = strings.TrimSuffix(base, "_completed")
	return base
}

// featureEntry pairs a human-readable display name with the UUID-based approval key.
type featureEntry struct {
	displayName string // filename without extension, e.g. "feature_003"
	approvalKey string // maggus-id UUID, or displayName as fallback
}

// listActiveFeatures returns entries for all active (non-completed) feature files in dir.
// Each entry contains the display name and the UUID-based approval key (with filename fallback).
func listActiveFeatures(dir string) ([]featureEntry, error) {
	files, err := parser.GlobFeatureFiles(dir, false)
	if err != nil {
		return nil, err
	}
	entries := make([]featureEntry, 0, len(files))
	for _, f := range files {
		name := featureIDFromPath(f)
		key := parser.ParseMaggusID(f)
		if key == "" {
			key = name
		}
		entries = append(entries, featureEntry{displayName: name, approvalKey: key})
	}
	return entries, nil
}

// resolveFeature finds a feature entry by display name (filename) and returns it.
// Returns the entry and true if found, or zero value and false otherwise.
func resolveFeature(dir, featureID string) (featureEntry, bool, error) {
	entries, err := listActiveFeatures(dir)
	if err != nil {
		return featureEntry{}, false, err
	}
	for _, e := range entries {
		if e.displayName == featureID {
			return e, true, nil
		}
	}
	return featureEntry{}, false, nil
}

var approveCmd = &cobra.Command{
	Use:          "approve [feature-id]",
	Short:        "Mark a feature as approved for execution",
	Long:         `Approve a feature so that maggus will work on it. When called with no argument, shows an interactive picker listing unapproved features.`,
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		if len(args) == 1 {
			return runApprove(cmd, dir, args[0])
		}
		return runApproveInteractive(cmd, dir)
	},
}

func runApprove(cmd *cobra.Command, dir, featureID string) error {
	entry, found, err := resolveFeature(dir, featureID)
	if err != nil {
		return err
	}
	if !found {
		cmd.PrintErrln(fmt.Sprintf("Error: feature %s not found", featureID))
		return fmt.Errorf("feature %s not found", featureID)
	}

	a, err := approval.Load(dir)
	if err != nil {
		return err
	}
	if a[entry.approvalKey] {
		cmd.Println(fmt.Sprintf("Feature %s is already approved", featureID))
		return nil
	}

	if err := approval.Approve(dir, entry.approvalKey); err != nil {
		return err
	}
	cmd.Println(fmt.Sprintf("Approved feature %s", featureID))
	return nil
}

// runApproveInteractive shows an interactive picker of unapproved features.
func runApproveInteractive(cmd *cobra.Command, dir string) error {
	entries, err := listActiveFeatures(dir)
	if err != nil {
		return err
	}

	a, err := approval.Load(dir)
	if err != nil {
		return err
	}

	// Filter to only unapproved features
	var unapproved []featureEntry
	for _, e := range entries {
		if !a[e.approvalKey] {
			unapproved = append(unapproved, e)
		}
	}

	if len(unapproved) == 0 {
		cmd.Println("All features are already approved.")
		return nil
	}

	displayNames := make([]string, len(unapproved))
	for i, e := range unapproved {
		displayNames[i] = e.displayName
	}

	selected, ok, err := runFeaturePicker("Select a feature to approve:", displayNames)
	if err != nil {
		return err
	}
	if !ok {
		cmd.Println("Cancelled.")
		return nil
	}
	return runApprove(cmd, dir, selected)
}

var unapproveCmd = &cobra.Command{
	Use:          "unapprove [feature-id]",
	Short:        "Revoke approval for a feature",
	Long:         `Unapprove a feature so that maggus will not work on it. When called with no argument, shows an interactive picker listing approved features.`,
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		if len(args) == 1 {
			return runUnapprove(cmd, dir, args[0])
		}
		return runUnapproveInteractive(cmd, dir)
	},
}

func runUnapprove(cmd *cobra.Command, dir, featureID string) error {
	entry, found, err := resolveFeature(dir, featureID)
	if err != nil {
		return err
	}
	if !found {
		cmd.PrintErrln(fmt.Sprintf("Error: feature %s not found", featureID))
		return fmt.Errorf("feature %s not found", featureID)
	}

	a, err := approval.Load(dir)
	if err != nil {
		return err
	}
	if !a[entry.approvalKey] {
		cmd.Println(fmt.Sprintf("Feature %s is not approved", featureID))
		return nil
	}

	if err := approval.Unapprove(dir, entry.approvalKey); err != nil {
		return err
	}
	cmd.Println(fmt.Sprintf("Unapproved feature %s", featureID))
	return nil
}

// runUnapproveInteractive shows an interactive picker of approved features.
func runUnapproveInteractive(cmd *cobra.Command, dir string) error {
	entries, err := listActiveFeatures(dir)
	if err != nil {
		return err
	}

	a, err := approval.Load(dir)
	if err != nil {
		return err
	}

	// Filter to only approved features
	var approved []featureEntry
	for _, e := range entries {
		if a[e.approvalKey] {
			approved = append(approved, e)
		}
	}

	if len(approved) == 0 {
		cmd.Println("No features are currently approved.")
		return nil
	}

	displayNames := make([]string, len(approved))
	for i, e := range approved {
		displayNames[i] = e.displayName
	}

	selected, ok, err := runFeaturePicker("Select a feature to unapprove:", displayNames)
	if err != nil {
		return err
	}
	if !ok {
		cmd.Println("Cancelled.")
		return nil
	}
	return runUnapprove(cmd, dir, selected)
}

// --- Interactive picker using bubbletea ---

// pickerModel is a simple bubbletea model for selecting a feature ID from a list.
type pickerModel struct {
	title     string
	items     []string
	cursor    int
	selected  string
	done      bool
	cancelled bool
}

var (
	pickerItemStyle     = lipgloss.NewStyle().PaddingLeft(2)
	pickerSelectedStyle = lipgloss.NewStyle().PaddingLeft(0).Bold(true).Foreground(lipgloss.Color("12"))
)

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.selected = m.items[m.cursor]
			m.done = true
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	var sb strings.Builder
	sb.WriteString(m.title + "\n\n")
	for i, item := range m.items {
		if i == m.cursor {
			sb.WriteString(pickerSelectedStyle.Render("> " + item))
		} else {
			sb.WriteString(pickerItemStyle.Render(item))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n(↑/↓ to move, Enter to select, Esc/q to cancel)")
	return sb.String()
}

// runFeaturePicker displays an interactive picker and returns the selected feature ID.
// Returns (selected, true, nil) on selection, or ("", false, nil) on cancellation.
func runFeaturePicker(title string, items []string) (string, bool, error) {
	m := pickerModel{
		title: title,
		items: items,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return "", false, fmt.Errorf("picker: %w", err)
	}
	final := result.(pickerModel)
	if final.cancelled || !final.done {
		return "", false, nil
	}
	return final.selected, true, nil
}

func init() {
	rootCmd.AddCommand(approveCmd)
	rootCmd.AddCommand(unapproveCmd)
}
