package phase

import "fmt"

var DefaultPipeline = []string{
	"prereq",
	"workspace",
	"skills",
	"reproduce",
	"plan",
	"risk-assessment",
	"fix",
	"verify",
	"test-coverage",
	"pr",
}

type Registry struct {
	phases map[string]Phase
	order  []string
}

func NewRegistry() *Registry {
	return &Registry{
		phases: make(map[string]Phase),
		order:  DefaultPipeline,
	}
}

func (r *Registry) Register(p Phase) {
	r.phases[p.Name()] = p
}

func (r *Registry) Get(name string) (Phase, bool) {
	p, ok := r.phases[name]
	return p, ok
}

func (r *Registry) Pipeline(from, to, only string, skip []string) ([]Phase, error) {
	skipSet := make(map[string]bool)
	for _, s := range skip {
		skipSet[s] = true
	}

	// Single phase mode
	if only != "" {
		p, ok := r.Get(only)
		if !ok {
			return nil, fmt.Errorf("unknown phase: %s", only)
		}
		return []Phase{p}, nil
	}

	// Determine start/end indices
	startIdx := 0
	endIdx := len(r.order) - 1

	if from != "" {
		found := false
		for i, name := range r.order {
			if name == from {
				startIdx = i
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown phase: %s", from)
		}
	}

	if to != "" {
		found := false
		for i, name := range r.order {
			if name == to {
				endIdx = i
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown phase: %s", to)
		}
	}

	if startIdx > endIdx {
		return nil, fmt.Errorf("--from phase '%s' comes after --to phase '%s'", from, to)
	}

	var result []Phase
	for i := startIdx; i <= endIdx; i++ {
		name := r.order[i]
		if skipSet[name] {
			continue
		}
		p, ok := r.Get(name)
		if !ok {
			continue // phase not registered (yet), skip
		}
		result = append(result, p)
	}

	return result, nil
}

func (r *Registry) PhaseNames() []string {
	var names []string
	for _, name := range r.order {
		if _, ok := r.phases[name]; ok {
			names = append(names, name)
		}
	}
	return names
}
