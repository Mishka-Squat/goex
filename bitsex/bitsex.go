package bitsex

import "math/bits"

const uintSize = 32 << (^uint(0) >> 63)

func MaskLeft(width uint) uint {
	return (1<<(width+1) - 1) << (uintSize - width)
}

func MaskRight(width uint) uint {
	return (1<<(width+1) - 1)
}

func HaveIncodedMask(n uint) int {
	lz := bits.LeadingZeros(^n)

	return lz
}
