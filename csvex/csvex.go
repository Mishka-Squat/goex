package csvex

import (
	"encoding/csv"
	"io"
	"iter"
	"reflect"
	"strings"

	"github.com/Mishka-Squat/orderedmap/v4"
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

type HeaderMap = orderedmap.Of[string, any]

func HeaderToMap(header []string) HeaderMap {
	m := HeaderMap(orderedmap.Make[string, any]())
	for _, h := range header {
		parts := strings.Split(h, ".")
		orderedmap.PathSet(m, none, parts...)
	}
	return m
}

func fieldsByName(rtype reflect.Type) map[string]reflect.StructField {
	fields := make(map[string]reflect.StructField, rtype.NumField())
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		fields[field.Name] = field
	}
	return fields
}

// joinIndex returns the reflect field-index path obtained by descending from
// path into index, e.g. joinIndex([]int{1}, []int{0}) is the path to the 0th
// field of the struct that is itself the field at index 1 of the root.
func joinIndex(path, index []int) []int {
	full := make([]int, 0, len(path)+len(index))
	full = append(full, path...)
	full = append(full, index...)
	return full
}

type CsvRow[T any] struct {
	Id string
	T  T
}

func ReadCsvTable[T any](r io.Reader) (items []CsvRow[T], err error) {
	reader := csv.NewReader(r)

	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	decoder := MakeDecoder[T](header)

	for row := range (*Reader)(reader).ReadAllSeq() {
		items = append(items, CsvRow[T]{
			Id: row[0],
			T:  decoder.Decode(row[1:]),
		})
	}

	return items, nil
}

func WriteCsvTable[T any](w io.Writer, header []string, items []CsvRow[T]) error {
	writer := csv.NewWriter(w)

	if err := writer.Write(append([]string{"id"}, header...)); err != nil {
		return err
	}

	encoder := MakeEncoder[T](header)

	row := make([]string, len(header)+1)
	for _, item := range items {
		row[0] = item.Id
		copy(row[1:], encoder.Encode(item.T))

		if err := writer.Write(row); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}
