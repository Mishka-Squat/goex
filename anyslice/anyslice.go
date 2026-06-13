package anyslice

type Slice struct{ any }

func Make(data ...any) Slice {
	switch len(data) {
	case 0:
		return Slice{nil}
	case 1:
		return Slice{data[0]}
	default:
		return Slice{data}
	}
}

func (s Slice) Len() int {
	switch a := s.any.(type) {
	case []any:
		return len(a)
	case any:
		return 1
	default:
		return 0
	}
}

func (s Slice) Nil() bool {
	return s.any == nil
}

func (s Slice) Slice() []any {
	switch a := s.any.(type) {
	case []any:
		return a
	case any:
		return []any{a}
	}
	return nil
}

func (s Slice) Append(data ...any) Slice {
	if len(data) == 0 {
		return s
	}

	switch a := s.any.(type) {
	case []any:
		return Slice{append(a, data...)}
	case any:
		return Slice{append([]any{a}, data...)}
	default:
		return Slice{data}
	}
}

func ByType[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

func (s Slice) FindFunc(fn func(v any) bool) (i int, v any) {
	for i, v = range s.Slice() {
		if fn(v) {
			return
		}
	}
	i = -1
	return
}

func FindType[T any](s Slice) (i int, t T) {
	for i, v := range s.Slice() {
		if t, ok := v.(T); ok {
			return i, t
		}
	}
	i = -1
	return
}

func (s *Slice) Set(i int, v any) {
	switch a := s.any.(type) {
	case []any:
		if i < len(a) {
			a[i] = v
		}
	case any:
		if i == 0 {
			s.any = v
		}
	}
}
