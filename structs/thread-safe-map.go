package structs

import "sync"

type MutexMap[V any] struct {
	mu    sync.RWMutex
	store map[string]V
}

func NewMutexMap[V any]() *MutexMap[V] {
	return &MutexMap[V]{
		store: make(map[string]V),
	}
}

func (m *MutexMap[V]) ContainsKey(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.store[key]
	return ok
}

func (m *MutexMap[V]) Put(key string, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.store[key] = value
}

// GetOrDefault returns the default zero value of the underlying type, if the value is not present
func (m *MutexMap[V]) GetOrDefault(key string) V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.store[key]
	if !ok {
		var zero V
		return zero
	}
	return v
}

func (m *MutexMap[V]) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
}

func (m *MutexMap[V]) Iterate(f func(key string, value V) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.store {
		if !f(k, v) {
			break
		}
	}
}

func (m *MutexMap[V]) ReadOnlyIterate(f func(key string, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.store {
		if !f(k, v) {
			break
		}
	}
}

func (m *MutexMap[V]) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.store)
}
