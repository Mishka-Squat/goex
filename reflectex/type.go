package reflectex

import (
	"iter"
	"reflect"
)

func TypeFields(t reflect.Type) iter.Seq[reflect.StructField] {
	return func(yield func(reflect.StructField) bool) {
		for i := 0; i < t.NumField(); i++ {
			if !yield(t.Field(i)) {
				break
			}
		}
	}
}
