package main

import (
	"time"
)

type RateMap[V any] struct {
	mp map[string]V
}

func NewMap[V any]() *RateMap[V] {
	return &RateMap[V]{mp: map[string]V{}}
}

func (m *RateMap[V]) Put(key string, value V) {
	m.mp[key] = value
}

func (m *RateMap[V]) Get(key string) (V, bool) {
	val, ok := m.mp[key]
	if !ok {
		var zeroValueV V
		return zeroValueV, false
	}
	return val, ok
}

func (m *RateMap[V]) Delete(key string) {
	delete(m.mp, key)
}

func (m *RateMap[V]) IsPresent(key string) bool {
	_, ok := m.mp[key]
	return ok
}

// Iterate applies the provided function to each entry in the SpeedMap.
func (m *RateMap[V]) Iterate(f func(peerId string, value V)) {
	for peerId, V := range m.mp {
		f(peerId, V)
	}
}

// Numeric constraint for summable types
type Numeric interface {
	~int | ~int64 | ~float32 | ~float64
}

// Sum method for numeric types
func Sum[V Numeric](m *RateMap[V]) V {
	var total V
	for _, value := range m.mp {
		total += value
	}
	return total
}

type SpeedMap = RateMap[float64]
type BytesMap = RateMap[int64]
type TimeMap = RateMap[time.Time]

func NewSpeedMap() *SpeedMap {
	return NewMap[float64]()
}

func NewBytesMap() *BytesMap {
	return NewMap[int64]()
}

func NewTimeMap() *TimeMap {
	return NewMap[time.Time]()
}
