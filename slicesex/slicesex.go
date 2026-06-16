package slicesex

import (
	"iter"
	"slices"

	"deedles.dev/xiter"
)

func Prepend[S []E, E any](s S, prefix ...E) S {
	return append(prefix, s...)
}

func Reserve[S ~[]E, E any](x S, size int) S {
	if len(x) < size {
		if size > cap(x) {
			x = slices.Grow(x, size)
		}

		x = x[0:size]
	}

	return x
}

func ReserveFill[S ~[]E, E any](x S, v E, size int) S {
	if len(x) < size {
		if size > cap(x) {
			x = slices.Grow(x, size)
		}

		ilen := len(x)
		x = x[0:size]
		for ; ilen < size; ilen++ {
			x[ilen] = v
		}
	}

	return x
}

func BinarySearchInsert[S ~[]E, E any](x S, item E, cmp func(E, E) int) S {
	i, _ := slices.BinarySearchFunc(x, item, cmp)
	return slices.Insert(x, i, item)
}

func BinarySearchTake[S ~[]E, E any](x S, item E, cmp func(E, E) int) S {
	i, _ := slices.BinarySearchFunc(x, item, cmp)
	return slices.Delete(x, i, 1)
}

// finds first element for which cmp(e) reurns true, removes it from slice and returns.
func TakeFunc[S ~[]E, E any](x S, cmp func(E) bool) (s S, e E, ok bool) {
	i := slices.IndexFunc(x, cmp)
	if i < 0 {
		ok = false
		s = x

		return
	}
	e = x[i]
	s = slices.Delete(x, i, 1)
	ok = true

	return
}

// Step over slice from shift starting index with specific period.
func Step[S ~[]E, E any](x S, shift, period int) []E {
	return slices.Collect(
		xiter.Step(slices.Values(x), shift, period),
	)
}

// take every even element from slice
func Even[S ~[]E, E any](x S) []E {
	return slices.Collect(
		xiter.Even(slices.Values(x)),
	)
}

// take every odd element from slice
func Odd[S ~[]E, E any](x S) []E {
	return slices.Collect(
		xiter.Odd(slices.Values(x)),
	)
}

// take first element from slice
func First[S ~[]E, E any](x S) E {
	if len(x) > 0 {
		return x[0]
	}

	var e E
	return e
}

// Special version for arrray iteration
func Any[S ~[]E, E any](x S, fn func(E) bool) bool {
	return xiter.Any(slices.Values(x), fn)
}

func All[S ~[]E, E any](x S, fn func(E) bool) bool {
	return xiter.All(slices.Values(x), fn)
}

func LeftJoin[S ~[]E, E, V any](x S, s iter.Seq[V]) iter.Seq2[E, V] {
	return func(yield func(E, V) bool) {
		i := 0
		for si := range s {
			if i == len(x) {
				return
			}

			if !yield(x[i], si) {
				return
			}

			i++
		}
	}
}

func Filter[S ~[]E, E any](x S, keep func(E) bool) []E {
	return xiter.CollectSize(xiter.Filter(slices.Values(x), keep), len(x))
}

func FilterSet[S ~[]E, E comparable, V any](x S, set map[E]V) []E {
	r := make([]E, 0, len(x))
	for _, i := range x {
		if _, ok := set[i]; ok {
			r = append(r, i)
		}
	}
	return r
}

func Map[S ~[]E, E, V any](x S, fn func(E) V) iter.Seq[V] {
	return xiter.Map(slices.Values(x), fn)
}

func Apply[S ~[]E, E any](x S, fn func(E) E) []E {
	for i := range x {
		x[i] = fn(x[i])
	}
	return x
}

func Transform[S ~[]E, E, V any](x S, fn func(E) V) []V {
	return xiter.CollectSize(xiter.Map(slices.Values(x), fn), len(x))
}

func ReplaceFunc[S ~[]E, E any](s S, v E, fn func(E) bool) ([]E, bool) {
	vi := slices.IndexFunc(s, fn)
	if vi == -1 {
		return s, false
	}
	s[vi] = v
	return s, true
}

func RepeatFunc[E any](count int, fn func(int) E) []E {
	return xiter.CollectSize(xiter.Map(Range(0, count), fn), count)
}

func Range(start int, count int) iter.Seq[int] {
	return xiter.Limit(xiter.Generate(start, 1), count)
}

func Split[T xiter.Comparable](start, end T, step T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range xiter.Generate(start, step) {
			if v > end {
				return
			}

			if !yield(v) {
				return
			}
		}
	}
}

func ToSet[V comparable](seq []V) map[V]struct{} {
	m := map[V]struct{}{}
	for _, k := range seq {
		m[k] = struct{}{}
	}
	return m
}

func ToMap[T any, K comparable](src []T, key func(T) K) map[K]T {
	var result = make(map[K]T)
	for _, v := range src {
		result[key(v)] = v
	}
	return result
}

func ToMapPtr[T any, K comparable](src []T, key func(*T) K) map[K]*T {
	var result = make(map[K]*T)
	for i := range src {
		result[key(&src[i])] = &src[i]
	}
	return result
}

func Set[S ~[]E, E any](x S, index int, e E) S {
	x = Reserve(x, index+1)
	x[index] = e
	return x
}

func RemoveSwapback[S ~[]E, E any](x S, index int) (S, E) {
	var e E

	if index >= len(x) {
		return x, e
	}

	e = x[index]
	x[index] = x[len(x)-1]
	x = x[:len(x)-1]

	return x, e
}

// casts sequence of one type to the sequence of the other type
// works only on sequences of interfaces.
func Cast[T any, S ~[]E, E any](x S) []T {
	t := make([]T, len(x))
	for i, xi := range x {
		if it, ok := (any)(xi).(T); ok {
			t[i] = it
		}
	}
	return t
}

// casts sequence of one type to the sequence of the other type
// works only on sequences of interfaces.
func CastSeq[T any, S ~[]E, E any](x S) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, i := range x {
			if it, ok := (any)(i).(T); ok {
				if !yield(it) {
					return
				}
			}
		}
	}
}
