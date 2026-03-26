package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/leberkas-org/maggus/internal/usage"
)

// Opus pricing approximation for cache savings calculation.
const (
	opusFullInputPricePerToken  = 15.0 / 1_000_000  // $15 per 1M input tokens
	opusCacheReadPricePerToken  = 1.50 / 1_000_000   // $1.50 per 1M cache read tokens
	opusCacheSavingPerToken     = opusFullInputPricePerToken - opusCacheReadPricePerToken
)

type modelStat struct {
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

type featureMetrics struct {
	tasksCompleted  int
	totalCostUSD    float64
	totalTokens     int64
	cacheHitRate    float64
	cacheSavingsUSD float64
	avgDurationSecs float64
	avgCostUSD      float64
	modelBreakdown  map[string]modelStat
}

type repoMetrics struct {
	featuresCompleted int
	bugsCompleted     int
	tasksCompleted    int
	totalCostUSD      float64
	totalTokens       int64
	gitCommits        int
}

// loadFeatureMetrics reads ~/.maggus/usage/work.jsonl, filters by item_id == itemID,
// and returns aggregated stats.
func loadFeatureMetrics(itemID string) featureMetrics {
	if itemID == "" {
		return featureMetrics{}
	}

	records := readWorkRecords()

	var fm featureMetrics
	fm.modelBreakdown = make(map[string]modelStat)

	var totalInput, totalCacheRead int64
	var totalDurationSecs float64
	var count int

	for _, r := range records {
		if r.ItemID != itemID {
			continue
		}
		count++
		fm.totalCostUSD += r.CostUSD
		fm.totalTokens += int64(r.InputTokens) + int64(r.OutputTokens) + int64(r.CacheCreationInputTokens) + int64(r.CacheReadInputTokens)
		totalInput += int64(r.InputTokens)
		totalCacheRead += int64(r.CacheReadInputTokens)

		dur := r.EndTime.Sub(r.StartTime).Seconds()
		if dur > 0 {
			totalDurationSecs += dur
		}

		// Aggregate per-model stats from ModelUsage map
		for model, mt := range r.ModelUsage {
			existing := fm.modelBreakdown[model]
			existing.InputTokens += int64(mt.InputTokens) + int64(mt.CacheCreationInputTokens) + int64(mt.CacheReadInputTokens)
			existing.OutputTokens += int64(mt.OutputTokens)
			existing.CostUSD += mt.CostUSD
			fm.modelBreakdown[model] = existing
		}
		// Fallback: if no ModelUsage, attribute to the record-level model
		if len(r.ModelUsage) == 0 && r.Model != "" {
			existing := fm.modelBreakdown[r.Model]
			existing.InputTokens += int64(r.InputTokens) + int64(r.CacheCreationInputTokens) + int64(r.CacheReadInputTokens)
			existing.OutputTokens += int64(r.OutputTokens)
			existing.CostUSD += r.CostUSD
			fm.modelBreakdown[r.Model] = existing
		}
	}

	fm.tasksCompleted = count

	if totalInput+totalCacheRead > 0 {
		fm.cacheHitRate = float64(totalCacheRead) / float64(totalInput+totalCacheRead)
	}

	fm.cacheSavingsUSD = float64(totalCacheRead) * opusCacheSavingPerToken

	if count > 0 {
		fm.avgDurationSecs = totalDurationSecs / float64(count)
		fm.avgCostUSD = fm.totalCostUSD / float64(count)
	}

	return fm
}

// loadRepoMetrics reads ~/.maggus/usage/work.jsonl, filters by repository == repoURL,
// and returns aggregated stats.
func loadRepoMetrics(repoURL string) repoMetrics {
	if repoURL == "" {
		return repoMetrics{}
	}

	records := readWorkRecords()

	var rm repoMetrics
	seenFeatures := make(map[string]bool)
	seenBugs := make(map[string]bool)

	for _, r := range records {
		if r.Repository != repoURL {
			continue
		}
		rm.tasksCompleted++
		rm.totalCostUSD += r.CostUSD
		rm.totalTokens += int64(r.InputTokens) + int64(r.OutputTokens) + int64(r.CacheCreationInputTokens) + int64(r.CacheReadInputTokens)

		// Count unique features/bugs by ItemShort prefix
		if r.ItemShort != "" {
			if strings.HasPrefix(r.ItemShort, "bug_") {
				seenBugs[r.ItemShort] = true
			} else {
				seenFeatures[r.ItemShort] = true
			}
		}
	}

	rm.featuresCompleted = len(seenFeatures)
	rm.bugsCompleted = len(seenBugs)

	// Git commits from global metrics
	globalDir, err := globalconfig.Dir()
	if err == nil {
		metrics, err := globalconfig.LoadMetricsFrom(filepath.Join(globalDir, "metrics.yml"))
		if err == nil {
			rm.gitCommits = int(metrics.GitCommits)
		}
	}

	return rm
}

// readWorkRecords reads all records from ~/.maggus/usage/work.jsonl.
func readWorkRecords() []usage.Record {
	globalDir, err := globalconfig.Dir()
	if err != nil {
		return nil
	}

	path := filepath.Join(globalDir, "usage", "work.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var records []usage.Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r usage.Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records
}

// loadMetrics refreshes the cached metrics for Tab 4 based on the currently selected plan.
func (m *statusModel) loadMetrics() {
	plan := m.selectedPlan()
	itemID := plan.ApprovalKey()
	repoURL := gitutil.RepoURL(m.dir)

	m.cachedFeatureMetrics = loadFeatureMetrics(itemID)
	m.cachedRepoMetrics = loadRepoMetrics(repoURL)

	globalDir, err := globalconfig.Dir()
	if err == nil {
		m.cachedGlobalMetrics, _ = globalconfig.LoadMetricsFrom(filepath.Join(globalDir, "metrics.yml"))
	}
}

// renderMetricsTab renders Tab 4: metrics sections for the selected feature,
// this repository, all-time global stats, and model breakdown.
func (m statusModel) renderMetricsTab(width, height int) string {
	fm := m.cachedFeatureMetrics
	rm := m.cachedRepoMetrics
	gm := m.cachedGlobalMetrics

	labelStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	valueStyle := lipgloss.NewStyle().Bold(true)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)

	var sb strings.Builder

	// ── Selected Feature ──
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("  Selected Feature"))
	sb.WriteString("\n")
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Tasks completed", fmt.Sprintf("%d", fm.tasksCompleted)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Total cost", runner.FormatCost(fm.totalCostUSD)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Total tokens", runner.FormatTokens(int(fm.totalTokens))))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Cache hit rate", fmt.Sprintf("%.1f%%", fm.cacheHitRate*100)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Cache savings", runner.FormatCost(fm.cacheSavingsUSD)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Avg duration", formatDurationSecs(fm.avgDurationSecs)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Avg cost/task", runner.FormatCost(fm.avgCostUSD)))

	// ── This Repository ──
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("  This Repository"))
	sb.WriteString("\n")
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Features", fmt.Sprintf("%d", rm.featuresCompleted)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Bugs", fmt.Sprintf("%d", rm.bugsCompleted)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Tasks completed", fmt.Sprintf("%d", rm.tasksCompleted)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Total cost", runner.FormatCost(rm.totalCostUSD)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Total tokens", runner.FormatTokens(int(rm.totalTokens))))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Git commits", fmt.Sprintf("%d", rm.gitCommits)))

	// ── All Time (Global) ──
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("  All Time (Global)"))
	sb.WriteString("\n")
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Work runs", fmt.Sprintf("%d", gm.WorkRuns)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Tasks completed", fmt.Sprintf("%d", gm.TasksCompleted)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Features completed", fmt.Sprintf("%d", gm.FeaturesCompleted)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Bugs completed", fmt.Sprintf("%d", gm.BugsCompleted)))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Total tokens", runner.FormatTokens(int(gm.TokensUsed))))
	sb.WriteString(metricsRow(labelStyle, valueStyle, "  Git commits", fmt.Sprintf("%d", gm.GitCommits)))

	// ── Model Breakdown ──
	if len(fm.modelBreakdown) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("  Model Breakdown"))
		sb.WriteString("\n")
		for model, stat := range fm.modelBreakdown {
			short := model
			// Shorten common model names
			if idx := strings.LastIndex(model, "/"); idx >= 0 {
				short = model[idx+1:]
			}
			detail := fmt.Sprintf("%s in / %s out · %s",
				runner.FormatTokens(int(stat.InputTokens)),
				runner.FormatTokens(int(stat.OutputTokens)),
				runner.FormatCost(stat.CostUSD))
			sb.WriteString(metricsRow(labelStyle, valueStyle, "  "+short, detail))
		}
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}

// metricsRow renders a single label + value row for the metrics grid.
func metricsRow(labelStyle, valueStyle lipgloss.Style, label, value string) string {
	const labelWidth = 22
	padded := label
	if len(label) < labelWidth {
		padded = label + strings.Repeat(" ", labelWidth-len(label))
	}
	return labelStyle.Render(padded) + " " + valueStyle.Render(value) + "\n"
}

// formatDurationSecs formats seconds into a human-readable duration string.
func formatDurationSecs(secs float64) string {
	if secs <= 0 {
		return "—"
	}
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	m := int(secs) / 60
	s := int(secs) % 60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh %dm", h, m)
}
