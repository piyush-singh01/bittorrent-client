package main

import (
	"sync"
	"time"
)

type ThreadSafeMap[V any] struct {
	mp sync.Map
}

func NewMap[V any]() *ThreadSafeMap[V] {
	return &ThreadSafeMap[V]{}
}

func (m *ThreadSafeMap[V]) Put(key string, value V) {
	m.mp.Store(key, value)
}

func (m *ThreadSafeMap[V]) Get(key string) (V, bool) {
	val, ok := m.mp.Load(key)
	if !ok {
		var zeroValueV V
		return zeroValueV, false
	}
	return val.(V), ok
}

func (m *ThreadSafeMap[V]) Delete(key string) {
	m.mp.Delete(key)
}

func (m *ThreadSafeMap[V]) IsPresent(key string) bool {
	_, ok := m.mp.Load(key)
	return ok
}

type SpeedMap = ThreadSafeMap[float64]
type BytesMap = ThreadSafeMap[int64]
type TimeMap = ThreadSafeMap[time.Time]

func NewSpeedMap() *SpeedMap {
	return NewMap[float64]()
}

func NewBytesMap() *BytesMap {
	return NewMap[int64]()
}

func NewTimeMap() *TimeMap {
	return NewMap[time.Time]()
}
