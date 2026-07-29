package maptree

import (
	"iter"

	"github.com/Mishka-Squat/goex"
)

// Of is a container with two type of node values - either V values or Of[K, V] values
// so that is basically a tree structure built round golang map implementation
type Of[K comparable, V any] map[K]any

func Make[V any, K comparable](v map[K]any) Of[K, V] {
	return Of[K, V](v)
}

func (m Of[K, V]) Root() map[K]any {
	return m
}

func (m Of[K, V]) IsEmpty() bool {
	return len(m) == 0
}

//func (m MapTree[K, V]) AllAny() iter.Seq2[K, any] {
//
//}

func (m Of[K, V]) Clone_() (r Of[K, V]) {
	r = Of[K, V]{}

	for k, v := range m {
		switch v := v.(type) {
		case map[K]any:
			r[k] = Of[K, V](v).Clone()
		case goex.Cloner:
			r[k] = v.Clone()
		default:
			r[k] = v
		}
	}

	return r
}

func (m Of[K, V]) Clone() any {
	return m.Clone_()
}

func (m Of[K, V]) Contains(path ...K) (ok bool) {
	r := m
	li := len(path) - 1
	for i, name := range path {
		if i == li {
			_, ok = r[name]
			return
		} else {
			r, ok = r.getNode(name)
			if !ok {
				return ok
			}
		}
	}

	return true // empty path is ture
}

func (m Of[K, V]) EnumObjects(path ...K) iter.Seq2[K, Of[K, V]] {
	if o, ok := m.GetNode(path...); ok {
		return func(yield func(K, Of[K, V]) bool) {
			for name, v := range o.Root() {
				if v, ok := m.toNode(v); ok {
					if !yield(name, v) {
						return
					}
				}
			}
		}
	}

	return func(yield func(K, Of[K, V]) bool) {}
}

func (m Of[K, V]) Get(path ...K) (v V, ok bool) {
	r := m
	li := len(path) - 1
	for i, name := range path {
		if i == li {
			if av, ok := r[name]; ok {
				if av == nil {
					var v V
					return v, ok
				}

				switch av := av.(type) {
				case V:
					return av, true
				}
			}

			ok = false
			return
		} else {
			r, ok = r.getNode(name)
			if !ok {
				return
			}
		}
	}

	ok = false
	return
}

func (m Of[K, V]) GetNode(path ...K) (v Of[K, V], ok bool) {
	r := m
	li := len(path) - 1
	if li < 0 {
		return m, true
	}
	for i, name := range path {
		if i == li {
			if v, ok = r.getNode(name); ok {
				return
			}

			ok = false
			return
		} else {
			r, ok = r.getNode(name)
			if !ok {
				return
			}
		}
	}

	v = Of[K, V]{}
	ok = false
	return
}

func (m Of[K, V]) GetAny(path ...K) (v any, ok bool) {
	r := m
	li := len(path) - 1
	for i, name := range path {
		if i == li {
			if v, ok = r[name]; ok {
				return
			}

			ok = false
			return
		} else {
			r, ok = r.getNode(name)
			if !ok {
				return
			}
		}
	}

	ok = false
	return
}

func (m Of[K, V]) Set(v V, path ...K) (ok bool) {
	if len(path) == 0 {
		return
	}

	r := m
	li := len(path) - 1
	for i, name := range path {
		if i == li {
			r[name] = v

			return true
		} else {
			if ar, ok := r[name]; ok {
				switch ar := ar.(type) {
				case Of[K, V]:
					r = ar
				default:
					return false // not a node can not traverse deeper
				}
			} else { // no node found, create new
				n := Of[K, V]{}
				r[name] = n
				r = n
			}
		}
	}

	return false
}

func (m Of[K, V]) SetNode(v Of[K, V], path ...K) (ok bool) {
	if len(path) == 0 {
		return
	}

	r := m
	li := len(path) - 1
	for i, name := range path {
		if i == li {
			r[name] = v

			return true
		} else {
			if ar, ok := r[name]; ok {
				switch ar := ar.(type) {
				case Of[K, V]:
					r = ar
				default:
					return false // not a node can not traverse deeper
				}
			} else { // no node found, create new
				n := Of[K, V]{}
				r[name] = n
				r = n
			}
		}
	}

	return false
}

func (m Of[K, V]) Merge(data Of[K, V]) (ok bool) {
	for k, v := range data {
		if mv, ok := m[k]; ok {
			data_node, okv := v.(Of[K, V])
			m_node, okmv := mv.(Of[K, V])

			if okv && okmv {
				m_node.Merge(data_node)
			} else if !okv && !okmv {
				m[k] = v
			} else {
				return false
			}
		} else {
			m[k] = v
		}
	}

	return true
}

func (m Of[K, V]) getNode(key K) (r Of[K, V], ok bool) {
	ar, ok := m[key]
	if !ok {
		return nil, false
	}

	return m.toNode(ar)
}

func (m Of[K, V]) toNode(v any) (r Of[K, V], ok bool) {
	switch v := v.(type) {
	case Of[K, V]:
		return v, true
	case map[K]any:
		return Of[K, V](v), true
	}

	return nil, false
}
