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

type validInterface interface {
	IsValid() bool
}

func MustValid[T validInterface](v T) T {
	if !v.IsValid() {
		panic("Not valid!!!")
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

func Whatever[T any, E any](v T, e E) T {
	return v
}

func PanicInt[T any](v T, e int) T {
	if e != -1 {
		return v
	}
	panic(e)
}

func OkInt[T any](v T, e int) (t T, ok bool) {
	if e != -1 {
		return v, true
	}
	return
}
