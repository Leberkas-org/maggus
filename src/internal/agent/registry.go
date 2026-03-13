package agent

import (
	"fmt"
	"sort"
	"strings"
)

// registry maps agent names to their constructor functions.
var registry = map[string]func() Agent{
	"claude":   func() Agent { return NewClaude() },
	"opencode": func() Agent { return NewOpenCode() },
}

// New creates an Agent by name. An empty name defaults to "claude"
// for backwards compatibility. Unknown names return an error listing
// available agents.
func New(name string) (Agent, error) {
	if name == "" {
		name = "claude"
	}

	ctor, ok := registry[name]
	if !ok {
		available := make([]string, 0, len(registry))
		for k := range registry {
			available = append(available, k)
		}
		sort.Strings(available)
		return nil, fmt.Errorf("unknown agent %q, available agents: %s", name, strings.Join(available, ", "))
	}

	return ctor(), nil
}
