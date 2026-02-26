package mathex

import (
	math "github.com/chewxy/math32"
	"golang.org/x/exp/constraints"
)

type SignedNumber interface {
	constraints.Signed | constraints.Float
}

type Number interface {
	SignedNumber | constraints.Unsigned
}

func NearZero[T Number](v T) bool {
	const s = 1e-8
	return math.Abs(float32(v)) < s
}

// Npot - Find neares power of two greater than v
func Npot[T Number](v T) T {
	r := 1
	for T(r) < v {
		r <<= 1
	}
	return T(r)
}

// Lerp - Calculate linear interpolation between two floats
func Lerp[T Number](time float32, start, end T) T {
	return T(float32(start) + time*float32(end-start))
}

// Normalize - Normalize input value within input range
func NormalizeF[T Number](value, start, end T) float32 {
	return float32(value-start) / float32(end-start)
}

// Normalize - Normalize input value within input range
func Normalize[T Number](value, start, end T) float32 {
	return float32(value-start) / float32(end-start)
}

// Remap - Remap input value within input range to output range
func Remap[T Number](value, inputStart, inputEnd, outputStart, outputEnd T) T {
	return Lerp(Normalize(value, inputStart, inputEnd), outputStart, outputEnd)
}

// Wrap - Wrap input value from min to max
func Wrap[T Number](value, min, max T) T {
	return value - (max-min)*T(math.Floor(Normalize(value, min, max)))
}

func Clamp[T Number](f, vmin, vmax T) T {
	return max(min(f, vmax), vmin)
}

func Clamp0[T Number](f, vmax T) T {
	return max(min(f, vmax), 0)
}

func Abs[T Number](v T) T {
	if v < 0 {
		return -v
	}
	return v
}

func Round[T Number](v T) T {
	return T(math.Round(float32(v)))
}

func RoundScale[T Number, F constraints.Float](v T, scale F) T {
	return T(math.Round(float32(F(v) * scale)))
}

func Ceil[T Number](v T) T {
	return T(math.Ceil(float32(v)))
}

func CeilScale[T Number, F constraints.Float](v T, scale F) T {
	return T(math.Ceil(float32(F(v) * scale)))
}

func Floor[T Number](v T) T {
	return T(math.Floor(float32(v)))
}

func FloorScale[T Number, F constraints.Float](v T, scale F) T {
	return T(math.Floor(float32(F(v) * scale)))
}

func Sqrt[T Number](v T) T {
	return T(math.Sqrt(float32(v)))
}

func Cos[T constraints.Float](v T) T {
	return T(math.Cos(float32(v)))
}

func Sin[T constraints.Float](v T) T {
	return T(math.Sin(float32(v)))
}

func Acos[T constraints.Float](v T) T {
	return T(math.Acos(float32(v)))
}

func Asin[T constraints.Float](v T) T {
	return T(math.Asin(float32(v)))
}

func Log[T Number](v T) T {
	return T(math.Log(float32(v)))
}

func Log10[T Number](v T) T {
	return T(math.Log10(float32(v)))
}

func Log2[T Number](v T) T {
	return T(math.Log2(float32(v)))
}

func Exp2[T Number](v T) T {
	return T(math.Exp2(float32(v)))
}

func Exp[T Number](v T) T {
	return T(math.Exp(float32(v)))
}

func Expm1[T Number](v T) T {
	return T(math.Expm1(float32(v)))
}
