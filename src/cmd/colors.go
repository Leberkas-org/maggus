package cmd

// Legacy ANSI color constants used by commands not yet migrated to lipgloss.
// These will be removed as each command is refactored (TASK-005, TASK-006).
const (
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBlue   = "\033[34m"
	colorDim    = "\033[2m"
	colorReset  = "\033[0m"
)
