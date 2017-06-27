package health

import "sync"

type Registry struct {
	mu     sync.RWMutex
	values map[string]*Value
}

func NewRegistry() *Registry {
	return &Registry{
		values: make(map[string]*Value),
	}
}

func (r *Registry) RegisterValue(name string) *Value {
	r.mu.Lock()
	defer r.mu.Unlock()

	v, ok := r.values[name]

	if !ok {
		v = &Value{}
		r.values[name] = v
		return v
	}

	return v
}

func (r *Registry) State() map[string]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := make(map[string]int64)

	for k, v := range r.values {
		state[k] = v.Number()
	}

	return state
}
