package timeex

import "time"

const unixToInternal = int64((1969*365 + 1969/4 - 1969/100 + 1969/400) * 24 * 60 * 60)

var EndOfTime = time.Unix(1<<63-1-unixToInternal, 999999999)

func Float32(dt time.Duration) float32 {
	return float32(dt) / float32(time.Second)
}
