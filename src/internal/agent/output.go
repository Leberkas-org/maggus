package agent

type OutputSink interface {
	OnStatus(status string)
	OnOutput(text string)
	OnTool(tool ToolEvent)
	OnUsage(usage UsageEvent)
	OnComplete(success bool)
}

type ToolEvent struct {
	Name  string
	Input string
}

type UsageEvent struct {
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheCreateTokens int
	CostUSD           float64
	Model             string
}
