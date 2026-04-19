package agent

import "fmt"

type Constructor func() Agent

type Registry struct {
	agents map[string]Constructor
}

func NewRegistry() *Registry {
	r := &Registry{agents: make(map[string]Constructor)}
	r.Register("claude", func() Agent { return &ClaudeAgent{} })
	r.Register("opencode", func() Agent { return &OpenCodeAgent{} })
	return r
}

func (r *Registry) Register(name string, ctor Constructor) {
	r.agents[name] = ctor
}

func (r *Registry) Get(name string) (Agent, error) {
	ctor, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("unknown agent: %q", name)
	}
	return ctor(), nil
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}
