package csvex

import (
	"reflect"
	"strconv"

	"github.com/Mishka-Squat/goex/gx"
	"github.com/Mishka-Squat/goex/stringsex"
	"github.com/Mishka-Squat/mengine/src/core/log"
)

// encOp always receives the *root* struct value (T). The reflect
// field-index path down to the (possibly nested) leaf field is baked into
// the closure at MakeEncoder time, so Encode just iterates Ops without
// needing to know anything about struct shape or nesting.
type encOp func(root reflect.Value, s *string) error

type Encoder struct {
	Ops []encOp
}

type EncoderT[T any] struct { // T is used for encode
	Encoder
}

type encoderMap map[reflect.Kind]encOp

var globalEncoders encoderMap = makeStandardEncoderMap(encoderMap{})

func makeStandardEncoderMap(m encoderMap) encoderMap {
	m[reflect.Bool] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatBool(v.Bool())
		return nil
	}
	m[reflect.Int] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatInt(v.Int(), 10)
		return nil
	}
	m[reflect.String] = func(v reflect.Value, s *string) error {
		*s = v.String()
		return nil
	}
	m[reflect.Float64] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatFloat(v.Float(), 'f', -1, 64)
		return nil
	}
	m[reflect.Float32] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatFloat(v.Float(), 'f', -1, 32)
		return nil
	}
	m[reflect.Int64] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatInt(v.Int(), 10)
		return nil
	}
	m[reflect.Int32] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatInt(v.Int(), 10)
		return nil
	}
	m[reflect.Int16] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatInt(v.Int(), 10)
		return nil
	}
	m[reflect.Int8] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatInt(v.Int(), 10)
		return nil
	}
	m[reflect.Uint] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatUint(v.Uint(), 10)
		return nil
	}
	m[reflect.Uint64] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatUint(v.Uint(), 10)
		return nil
	}
	m[reflect.Uint32] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatUint(v.Uint(), 10)
		return nil
	}
	m[reflect.Uint16] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatUint(v.Uint(), 10)
		return nil
	}
	m[reflect.Uint8] = func(v reflect.Value, s *string) error {
		*s = strconv.FormatUint(v.Uint(), 10)
		return nil
	}
	// m[reflect.Struct] = llok at  MakeDecoderAny

	return m
}

func MakeEncoder[T any](header []string) *EncoderT[T] {
	hmap := HeaderToMap(header)
	rtype := reflect.TypeFor[T]()

	return &EncoderT[T]{
		Encoder: MakeEncoderAny(rtype, hmap),
	}
}

type CsvFormatter interface {
	String() string
}

// Builds a transformer from rtype struct to header difned row
func MakeEncoderAny(rtype reflect.Type, header HeaderMap) Encoder {
	var e Encoder
	appendEncodeOps(&e, rtype, header, nil)
	return e
}

// appendEncodeOps walks rtype/header, appending one encOp per leaf (scalar
// or CsvFormatter) field to e.Ops. path is the field-index path, relative to
// the root struct, of rtype itself, so ops built for nested structs bake in
// the full path down from the root rather than just their local field index.
func appendEncodeOps(e *Encoder, rtype reflect.Type, header HeaderMap, path []int) {
	fields := fieldsByName(rtype)

	for name, header := range header.All() {
		name = stringsex.Title(name)
		field, ok := fields[name]
		if !ok {
			continue // TODO: handle error
		}
		fieldPath := joinIndex(path, field.Index)

		if header, ok := header.(HeaderMap); ok {
			if field.Type.Kind() != reflect.Struct {
				continue // TODO: handle error
			}
			appendEncodeOps(e, field.Type, header, fieldPath)
			continue
		}

		if op, ok := globalEncoders[field.Type.Kind()]; ok {
			e.Ops = append(e.Ops, func(root reflect.Value, s *string) error {
				return op(root.FieldByIndex(fieldPath), s)
			})
			continue
		}

		formatterType := reflect.TypeFor[CsvFormatter]()
		if !field.Type.Implements(formatterType) {
			log.Fatal("wht")
			continue
		}

		e.Ops = append(e.Ops, func(root reflect.Value, s *string) error {
			v := root.FieldByIndex(fieldPath)
			method := gx.MustValid(v.MethodByName("String"))
			rv := method.Call(nil)
			*s = rv[0].String()

			return nil
		})
	}
}

func (e EncoderT[T]) Encode(item T) []string {
	v := reflect.ValueOf(&item).Elem()
	row := make([]string, len(e.Ops))

	for i, op := range e.Ops {
		_ = op(v, &row[i]) // TODO: handle error
	}

	return row
}
