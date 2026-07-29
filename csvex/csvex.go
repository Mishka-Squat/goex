package csvex

import (
	"encoding/csv"
	"iter"
)

type Reader csv.Reader

func (r *Reader) ReadAllSeq() iter.Seq[[]string] {
	return func(yeild func([]string) bool) {
		for {
			record, err := (*csv.Reader)(r).Read()
			if err != nil {
				return
			}
			if !yeild(record) {
				return
			}
		}
	}
}
