package runtimeex

import (
	"fmt"
	"runtime"
)

func GetFunctionName() string {
	return GetCallerName(1)
}

func GetCallerName(skip int) (name string) {
	name = "<unknown>"
	pc, _, _, ok := runtime.Caller(1 + skip)
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			name = fn.Name()
		}
	}

	return
}

func MeasureMemory() (fn func()) {
	fnName := GetCallerName(1)

	startMemStats := runtime.MemStats{}
	fn = func() {
		endMemStats := runtime.MemStats{}
		runtime.ReadMemStats(&endMemStats)

		mallocs := endMemStats.Mallocs - startMemStats.Mallocs
		if mallocs != 0 {
			fmt.Printf("%s did %d memory allocations\n", fnName, mallocs)
		}
	}

	runtime.ReadMemStats(&startMemStats)
	return
}
