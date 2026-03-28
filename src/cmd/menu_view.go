package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

const logo = `
 ███╗   ███╗  █████╗   ██████╗  ██████╗  ██╗   ██╗ ███████╗
 ████╗ ████║ ██╔══██╗ ██╔════╝ ██╔════╝  ██║   ██║ ██╔════╝
 ██╔████╔██║ ███████║ ██║  ███╗██║  ███╗ ██║   ██║ ███████╗
 ██║╚██╔╝██║ ██╔══██║ ██║   ██║██║   ██║ ██║   ██║ ╚════██║
 ██║ ╚═╝ ██║ ██║  ██║ ╚██████╔╝╚██████╔╝ ╚██████╔╝ ███████║
 ╚═╝     ╚═╝ ╚═╝  ╚═╝  ╚═════╝  ╚═════╝   ╚═════╝  ╚══════╝`

func (m menuModel) View() string {
	themeColor := styles.ThemeColor(m.is2x)
	logoStyle := lipgloss.NewStyle().Foreground(themeColor).Bold(true)
	versionStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	versionLine := versionStyle.Render(fmt.Sprintf("v%s — Markdown Agent for Goal-Gated Unsupervised Sprints", Version))

	// Feature & bug summary line (pre-styled by formatSummaryLine)
	summaryLine := formatSummaryLine(m.summary)

	var body, footer string
	if m.confirmStopDaemon {
		body = m.viewConfirmStopDaemon()
		footer = styles.StatusBar.Render("y: stop daemon · N/enter: keep running")
	} else if m.inSubMenu {
		body, footer = m.viewSubMenu()
	} else {
		body, footer = m.viewMainMenu()
	}

	// Center the logo, version, and summary lines within the content column.
	// FullScreen left-pads all content into a maxContentWidth (90) column,
	// so center relative to that width, not the full inner width.
	const contentW = 90
	styledLogo := logoStyle.Render(logo)
	header := centerBlock(styledLogo, contentW) + "\n" +
		centerLine(versionLine, contentW) + "\n" +
		centerLine(summaryLine, contentW)

	// Show current working directory below the summary
	if m.cwd != "" {
		cwdStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
		cwdDisplay := m.cwd
		// Only truncate if this is a git repo and not the home directory.
		if home, err := os.UserHomeDir(); err != nil || (m.cwd != home && isGitRepoCheck(m.cwd)) {
			cwdDisplay = truncateLeft(m.cwd, contentW-4)
		}
		header += "\n" + centerLine(cwdStyle.Render(cwdDisplay), contentW)
	}

	// Daemon status line
	header += "\n" + centerLine(formatDaemonStatusLine(m.daemon), contentW)

	// Show 2x remaining time below the summary when active
	if m.is2x && m.twoXExpiresIn != "" {
		twoXStyle := lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)
		twoXLine := twoXStyle.Render(fmt.Sprintf("2x expires in: %s", m.twoXExpiresIn))
		header += "\n" + centerLine(twoXLine, contentW)
	}

	// Show update banner when available
	if m.updateBanner != "" {
		updateStyle := lipgloss.NewStyle().Foreground(styles.Success).Bold(true)
		header += "\n" + centerLine(updateStyle.Render(m.updateBanner), contentW)
	}

	// Show daemon auto-start warning (non-fatal)
	if m.daemonAutoWarning != "" {
		warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)
		header += "\n" + centerLine(warnStyle.Render(m.daemonAutoWarning), contentW)
	}

	content := header + "\n\n" + body

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenColor(content, footer, m.width, m.height, themeColor)
	}
	return styles.Box.BorderForeground(themeColor).Render(content+"\n\n"+footer) + "\n"
}

// centerLine centers a single line of text within the given width.
func centerLine(line string, width int) string {
	w := lipgloss.Width(line)
	pad := (width - w) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + line
}

// centerBlock centers each line of a multi-line string independently.
func centerBlock(block string, width int) string {
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	for i, line := range lines {
		lines[i] = centerLine(line, width)
	}
	return strings.Join(lines, "\n")
}

// highlightShortcut renders the name with the shortcut character underlined
// when active is true. Otherwise renders the full name with the base style.
func highlightShortcut(name string, shortcut rune, base lipgloss.Style, active bool) string {
	if !active || shortcut == 0 {
		return base.Render(name)
	}
	underline := base.Underline(true)
	for i, ch := range name {
		if ch == shortcut {
			before := name[:i]
			after := name[i+len(string(ch)):]
			return base.Render(before) + underline.Render(string(ch)) + base.Render(after)
		}
	}
	return base.Render(name)
}

// truncateLeft truncates a path from the left, adding "..." prefix.
func truncateLeft(path string, maxWidth int) string {
	if maxWidth <= 0 || len(path) <= maxWidth {
		return path
	}
	if maxWidth <= 3 {
		return path[len(path)-maxWidth:]
	}
	return "..." + path[len(path)-(maxWidth-3):]
}

// viewConfirmStopDaemon renders the "Stop daemon?" confirmation prompt.
func (m menuModel) viewConfirmStopDaemon() string {
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Warning)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	boldStyle := lipgloss.NewStyle().Bold(true)

	var sb strings.Builder
	sb.WriteString(warnStyle.Render("Stop daemon?") + " " + mutedStyle.Render("[y/N]"))
	sb.WriteString("\n\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf("  The daemon is running (PID %d).", m.daemon.PID)))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  %s / %s",
		boldStyle.Render("y: stop daemon and exit"),
		mutedStyle.Render("N/enter/esc: exit without stopping"),
	))
	return sb.String()
}

func (m menuModel) viewMainMenu() (string, string) {
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	descStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()

	// Measure columns: left = "→ " + name, right = desc
	maxNameW := 0
	maxDescW := 0
	for _, item := range m.items {
		if len(item.name) > maxNameW {
			maxNameW = len(item.name)
		}
		if len(item.desc) > maxDescW {
			maxDescW = len(item.desc)
		}
	}

	// Total row width: cursor(4) + name(maxNameW) + gap(2) + desc(maxDescW)
	// cursor column is "  → " (4 chars) for selected, "    " (4 chars) for others
	const cursorCol = 4
	const gap = 2
	tableW := cursorCol + maxNameW + gap + maxDescW

	// Center the table within the content column (90 chars, matching FullScreen)
	const contentW = 90
	leftPad := (contentW - tableW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	indent := strings.Repeat(" ", leftPad)

	var sb strings.Builder
	for i, item := range m.items {
		if item.separator {
			sb.WriteString("\n")
		}
		// Right-align the name within the column.
		padded := fmt.Sprintf("%*s", maxNameW, item.name)
		if i == m.cursor {
			nameStyle := selectedStyle
			cursor := cursorStyle
			if item.isExit {
				nameStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
				cursor = nameStyle
			}
			fmt.Fprintf(&sb, "%s%s %s  %s\n",
				indent,
				cursor.Render("→"),
				highlightShortcut(padded, item.shortcut, nameStyle, m.showShortcuts),
				descStyle.Render(item.desc),
			)
		} else {
			fmt.Fprintf(&sb, "%s  %s  %s\n",
				indent,
				highlightShortcut(padded, item.shortcut, normalStyle, m.showShortcuts),
				descStyle.Render(item.desc),
			)
		}
	}

	footer := styles.StatusBar.Render("↑/↓ navigate · enter select · hold alt for shortcuts · q: exit")
	return sb.String(), footer
}

func (m menuModel) viewSubMenu() (string, string) {
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()
	activeValueStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)

	cmdName := m.items[m.cursor].name
	titleLine := selectedStyle.Render(cmdName) + "  " + mutedStyle.Render(m.items[m.cursor].desc)

	// Measure the widest row to center the sub-menu table.
	// Row structure: cursor(4) + label(10) + gap(2) + values
	const cursorCol = 4
	const labelCol = 10
	const gap = 2
	maxValuesW := 0
	for _, opt := range m.activeSubDef.options {
		// Measure raw (unstyled) values width: "v1 / v2 / v3"
		valW := 0
		for vi, v := range opt.values {
			if vi > 0 {
				valW += 3 // " / "
			}
			valW += len(v)
		}
		if valW > maxValuesW {
			maxValuesW = valW
		}
	}
	tableW := cursorCol + labelCol + gap + maxValuesW

	// Also account for the title line width
	titleW := len(cmdName) + 2 + len(m.items[m.cursor].desc)
	if titleW > tableW {
		tableW = titleW
	}

	const contentW = 90
	leftPad := (contentW - tableW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	indent := strings.Repeat(" ", leftPad)

	var sb strings.Builder
	sb.WriteString(indent + titleLine + "\n")
	sb.WriteString(indent + styles.Separator(tableW) + "\n")

	for i, opt := range m.activeSubDef.options {
		label := fmt.Sprintf("%-10s", opt.label)

		// Render value choices
		var valueParts []string
		for vi, v := range opt.values {
			if vi == opt.current {
				valueParts = append(valueParts, activeValueStyle.Render(v))
			} else {
				valueParts = append(valueParts, mutedStyle.Render(v))
			}
		}
		valueStr := strings.Join(valueParts, mutedStyle.Render(" / "))

		if i == m.subCursor {
			fmt.Fprintf(&sb, "%s  %s %s  %s\n",
				indent,
				cursorStyle.Render("→"),
				normalStyle.Render(label),
				valueStr,
			)
		} else {
			fmt.Fprintf(&sb, "%s    %s  %s\n",
				indent,
				normalStyle.Render(label),
				valueStr,
			)
		}
	}

	// Run item
	runIdx := len(m.activeSubDef.options)
	sb.WriteString("\n")
	if m.subCursor == runIdx {
		fmt.Fprintf(&sb, "%s  %s %s\n",
			indent,
			cursorStyle.Render("→"),
			selectedStyle.Render("Run"),
		)
	} else {
		fmt.Fprintf(&sb, "%s    %s\n",
			indent,
			normalStyle.Render("Run"),
		)
	}

	footer := styles.StatusBar.Render("↑/↓: navigate · ←/→: change value · enter: select/run · q: back")
	return sb.String(), footer
}
