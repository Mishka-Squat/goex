package gx

func IsNil[T comparable](v T) bool {
	var n T
	return v == n
}

func Ok(e error) bool {
	return e == nil
}

func Check[T any](v T, e error) (T, bool) {
	return v, e == nil
}

func Must[T any](v T, e error) T {
	if e != nil {
		panic(e)
	}
	return v
}

func Should[T any](v T, e error) T {
	if e != nil {
		var d T
		return d
	}
	return v
}

func MustHave[T any](v T, ok bool) T {
	if !ok {
		panic(ok)
	}
	return v
}

func ShouldHave[T any](v T, ok bool) T {
	if ok {
		return v
	}
	var t T
	return t
}
