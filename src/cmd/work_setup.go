package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/fingerprint"
	"github.com/leberkas-org/maggus/internal/gitignore"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/spf13/cobra"
)

// workConfig holds all resolved configuration needed by the work loop.
type workConfig struct {
	count           int
	dir             string
	cfg             config.Config
	validIncludes   []string
	includeWarnings []string
	activeAgent     agent.Agent
	resolvedModel   string
	modelDisplay    string
	notifier        *notify.Notifier
	useWorktree     bool
	hostFingerprint string
}

// workSetup resolves CLI flags, loads config, validates the agent, and
// prepares all configuration the work loop needs to run.
func workSetup(cmd *cobra.Command, args []string) (*workConfig, error) {
	count := countFlag

	if taskFlag != "" {
		count = 1
	} else if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, fmt.Errorf("invalid task count %q: must be a positive integer", args[0])
		}
		if n <= 0 {
			return nil, fmt.Errorf("task count must be a positive integer, got %d", n)
		}
		count = n
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	// Load config
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Validate includes: collect warnings for missing files, skip them from prompt
	validIncludes := config.ValidateIncludes(cfg.Include, dir)
	var includeWarnings []string
	for _, inc := range cfg.Include {
		found := false
		for _, v := range validIncludes {
			if v == inc {
				found = true
				break
			}
		}
		if !found {
			includeWarnings = append(includeWarnings, fmt.Sprintf("Warning: included file not found: %s", inc))
		}
	}

	// Resolve agent: CLI flag > config > default ("claude")
	agentName := cfg.Agent
	if agentFlag != "" {
		agentName = agentFlag
	}
	activeAgent, err := agent.New(agentName)
	if err != nil {
		return nil, err
	}

	// Validate agent CLI is installed before starting work
	if err := activeAgent.Validate(); err != nil {
		return nil, fmt.Errorf("agent %q not available: %w", activeAgent.Name(), err)
	}

	// Resolve model: CLI flag overrides config file
	modelInput := cfg.Model
	if modelFlag != "" {
		modelInput = modelFlag
	}
	resolvedModel := config.ResolveModel(modelInput)

	// Create notifier for sound notifications.
	notifier := notify.New(cfg.Notifications)

	// Resolve worktree mode: --no-worktree > --worktree > config > default (false)
	useWorktree := cfg.Worktree
	if worktreeFlag {
		useWorktree = true
	}
	if noWorktreeFlag {
		useWorktree = false
	}

	// Ensure .gitignore has required entries
	if _, err := gitignore.EnsureEntries(dir); err != nil {
		return nil, fmt.Errorf("check gitignore: %w", err)
	}

	// Get host fingerprint
	hostFingerprint, _ := fingerprint.Get()
	if hostFingerprint == "" {
		hostFingerprint = "unknown"
	}

	modelDisplay := resolvedModel
	if modelDisplay == "" {
		modelDisplay = "default"
	}

	return &workConfig{
		count:           count,
		dir:             dir,
		cfg:             cfg,
		validIncludes:   validIncludes,
		includeWarnings: includeWarnings,
		activeAgent:     activeAgent,
		resolvedModel:   resolvedModel,
		modelDisplay:    modelDisplay,
		notifier:        notifier,
		useWorktree:     useWorktree,
		hostFingerprint: hostFingerprint,
	}, nil
}
