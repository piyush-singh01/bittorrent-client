package structs

import "sync"

type MutexMap[K key, V any] struct {
	mu    sync.RWMutex
	store map[K]V
}

func NewMutexMap[K key, V any]() *MutexMap[K, V] {
	return &MutexMap[K, V]{
		store: make(map[K]V),
	}
}

func (m *MutexMap[K, V]) ContainsKey(key K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.store[key]
	return ok
}

func (m *MutexMap[K, V]) Put(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.store[key] = value
}

func (m *MutexMap[K, V]) PutOnlyIfNotExists(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.store[key]; !ok {
		m.store[key] = value
	}
}

// GetOrDefault returns the default zero value of the underlying type, if the value is not present
func (m *MutexMap[K, V]) GetOrDefault(key K) V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.store[key]
	if !ok {
		var zero V
		return zero
	}
	return v
}

func (m *MutexMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
}

func (m *MutexMap[K, V]) Iterate(f func(key K, value V) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.store {
		if !f(k, v) {
			break
		}
	}
}

func (m *MutexMap[K, V]) ReadOnlyIterate(f func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.store {
		if !f(k, v) {
			break
		}
	}
}

func (m *MutexMap[K, V]) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.store)
}
