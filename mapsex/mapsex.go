package mapsex

import (
	"cmp"
	"iter"
	"maps"
	"slices"

	"deedles.dev/xiter"
)

func CollectSeq[K comparable, V any](seq iter.Seq[V], keyFn func(V) K) map[K]V {
	r := map[K]V{}
	for i := range seq {
		r[keyFn(i)] = i
	}
	return r
}

func CollectSet[V comparable](seq iter.Seq[V]) map[V]struct{} {
	m := map[V]struct{}{}
	for k := range seq {
		m[k] = struct{}{}
	}
	return m
}

// Gets first element from map
func First[K comparable, V any](m map[K]V) (k K, v V, ok bool) {
	for k, v = range m {
		ok = true
		return
	}

	ok = false
	return
}

// Gets only element from map
func Only[K comparable, V any](m map[K]V) (k K, v V, ok bool) {
	if len(m) == 1 {
		for k, v = range m {
			ok = true
			return
		}
	}

	ok = false
	return
}

func SortedValues[K comparable, V any](m map[K]V, less func(a, b V) int) (vals []V) {
	vals = slices.Collect(maps.Values(m))
	slices.SortFunc(vals, less)
	return
}

func IndexedSlice[V comparable](x []V) map[V]int {
	fw := map[V]int{}
	for i, f := range x {
		fw[f] = i
	}
	return fw
}

func IndexedSeq[V comparable](x iter.Seq[V]) map[V]int {
	fw := map[V]int{}
	for i, f := range xiter.Enumerate(x) {
		fw[f] = i
	}
	return fw
}

//func DeepCopy[M1 ~map[K]V, M2 ~map[K]V, K comparable, V any](dst M1, src M2) {
//	for k, v := range src {
//		var av any = v
//		switch vv := av.(type) {
//		case map[K]V:
//
//		}
//	}
//}

// Append value to multimap
func AppendMultiMap[K comparable, V any](m map[K][]V, key K, value V) map[K][]V {
	if a, ok := m[key]; ok {
		m[key] = append(a, value)
	} else {
		m[key] = []V{value}
	}

	return m
}

// Delete value from multimap
//func DeleteMultiMap[K comparable, V any](m map[K][]V, key K) (v V) {
//	if a, ok := m[key]; ok {
//		if len(a) > 1 {
//			m[key] = a[1:]
//		} else {
//			delete(m, key)
//		}
//	}
//
//	return m
//}

// Append value to multimap
func InsertKeys[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	if m == nil {
		m = map[K]V{}
	}

	for _, key := range keys {
		var v V
		m[key] = v
	}

	return m
}

func InsertKeysSeq[K comparable, V any](m map[K]V, keys iter.Seq[K]) map[K]V {
	if m == nil {
		m = map[K]V{}
	}

	for key := range keys {
		var v V
		m[key] = v
	}

	return m
}

func Combine[K comparable, V any](m1, m2 map[K]V) bool {
	if len(m2) > len(m1) {
		m1, m2 = m2, m1
	}

	for k := range m2 {
		if _, ok := m1[k]; ok {
			return true
		}
	}

	return false
}

func Intersect[K comparable, V any](m1, m2 map[K]V) bool {
	if len(m2) > len(m1) {
		m1, m2 = m2, m1
	}

	for k := range m2 {
		if _, ok := m1[k]; ok {
			return true
		}
	}

	return false
}

func SortedByKeySeq[K interface {
	comparable
	cmp.Ordered
}, V any](m map[K]V) iter.Seq2[K, V] {
	keys := slices.Collect(maps.Keys(m))
	slices.Sort(keys)

	return func(yield func(K, V) bool) {
		for _, k := range keys {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}
