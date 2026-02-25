package errorsex

import "errors"

type matchResult int

const (
	matchNext matchResult = iota
	matchError
	matchRecover
)

func Switch(err error, matches ...func(error) matchResult) bool {
	for _, m := range matches {
		switch m(err) {
		case matchError:
			return true
		case matchRecover:
			return false
		}
	}

	return false
}

func Match[E error](fn func(E)) func(error) matchResult {
	return func(err error) matchResult {
		if err, ok := errors.AsType[E](err); ok {
			fn(err)
			return matchError
		}

		return matchNext
	}
}

func MatchOk[E error](fn func(E) bool) func(error) matchResult {
	return func(err error) matchResult {
		if err, ok := errors.AsType[E](err); ok {
			if fn(err) {
				return matchRecover
			}

			return matchError
		}

		return matchNext
	}
}
