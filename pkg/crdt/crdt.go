package crdt

import "maps"

type CRDT[T any, S any] interface {
	Value() T
	State() S
	Merge(other S) S
}

type LWW[T any] struct {
	id        string
	timestamp int
	value     T
	tombstone bool
}

var _ CRDT[string, *LWW[string]] = &LWW[string]{}

func NewLWW[T any](id string, value T) *LWW[T] {
	return &LWW[T]{
		id:        id,
		timestamp: 0,
		value:     value,
	}
}

func (lww *LWW[T]) Value() T {
	if lww.tombstone {
		var t T
		return t
	}
	return lww.value
}

func (lww *LWW[T]) IsDead() bool {
	return lww.tombstone
}

func (lww *LWW[T]) State() *LWW[T] {
	return lww
}

func (lww *LWW[T]) Merge(other *LWW[T]) *LWW[T] {
	if other.timestamp > lww.timestamp {
		return other
	} else if other.timestamp == lww.timestamp && other.id > lww.id {
		return other
	}
	return lww
}

func (lww *LWW[T]) Set(v T) {
	lww.timestamp += 1
	lww.value = v
	lww.tombstone = false
}

func (lww *LWW[T]) Delete() {
	lww.timestamp += 1
	lww.tombstone = true
}

type LWWMap[T any] map[string]*LWW[T]

var _ CRDT[map[string]string, LWWMap[string]] = LWWMap[string]{}

func NewLWWMap[T any]() LWWMap[T] {
	return LWWMap[T]{}
}

func (lwwm LWWMap[T]) Value() map[string]T {
	out := make(map[string]T, len(lwwm))
	for k, v := range lwwm {
		if lwwm.Has(k) {
			out[k] = v.Value()
		}
	}
	return out
}

func (lwwm LWWMap[T]) State() LWWMap[T] {
	return lwwm
}

func (lwwm LWWMap[T]) Merge(other LWWMap[T]) LWWMap[T] {
	out := LWWMap[T]{}
	maps.Copy(out, lwwm)
	for k, v := range other {
		if existing, exists := out[k]; exists {
			out[k] = existing.Merge(v)
		} else {
			out[k] = v
		}
	}
	return lwwm
}

func (lwwm LWWMap[T]) Set(k string, v T) {
	lwwm[k].Set(v)
}

func (lwwm LWWMap[T]) Get(k string) {
	lwwm[k].Value()
}

func (lwwm LWWMap[T]) Has(k string) bool {
	if lww, ok := lwwm[k]; ok && !lww.IsDead() {
		return true
	}
	return false
}
