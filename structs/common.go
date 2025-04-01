package structs

import "strings"

type number interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64
}

type key interface {
	number | string
}

type hashSet[K key] struct {
	set map[K]struct{}
}

func (h *hashSet[K]) String() string {
	res := make([]string, len(h.set))
	for k := range h.set {
		res = append(res, string(k))
	}
	return strings.Join(res, ",")
}

func newHashSet[K key]() *hashSet[K] {
	return &hashSet[K]{make(map[K]struct{})}
}

func (h *hashSet[K]) insert(key K) {
	h.set[key] = struct{}{}
}

func (h *hashSet[K]) delete(key K) {
	delete(h.set, key)
}

func (h *hashSet[K]) contains(key K) bool {
	_, ok := h.set[key]
	return ok
}

func (h *hashSet[K]) getAny() K {
	var zero K
	for k := range h.set {
		return k
	}
	return zero
}

func (h *hashSet[K]) size() int {
	return len(h.set)
}

func (h *hashSet[K]) isEmpty() bool {
	return len(h.set) == 0
}
