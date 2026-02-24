package agent

import "sync"

// AgentRegistry manages multiple AgentInstances and provides lookup by ID.
type AgentRegistry struct {
	agents    map[string]*AgentInstance
	defaultID string
	mu        sync.RWMutex
}

func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]*AgentInstance),
	}
}

// Register adds an agent instance. The first agent registered with Default=true
// (or the very first agent) becomes the default.
func (r *AgentRegistry) Register(inst *AgentInstance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[inst.ID] = inst
	if r.defaultID == "" {
		r.defaultID = inst.ID
	}
}

// SetDefault explicitly sets the default agent ID.
func (r *AgentRegistry) SetDefault(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultID = id
}

// Get returns an agent by ID.
func (r *AgentRegistry) Get(id string) (*AgentInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.agents[id]
	return inst, ok
}

// GetDefault returns the default agent.
func (r *AgentRegistry) GetDefault() *AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[r.defaultID]
}

// List returns all registered agent instances.
func (r *AgentRegistry) List() []*AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*AgentInstance, 0, len(r.agents))
	for _, inst := range r.agents {
		result = append(result, inst)
	}
	return result
}

// ListIDs returns the IDs of all registered agents.
func (r *AgentRegistry) ListIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of registered agents.
func (r *AgentRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}
