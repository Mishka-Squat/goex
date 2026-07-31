package csvex

import (
	"reflect"
	"strconv"

	"github.com/Mishka-Squat/goex/gx"
	"github.com/Mishka-Squat/mengine/src/core/log"
	"github.com/iancoleman/strcase"
)

// decOp always receives the *root* struct value (T). The reflect
// field-index path down to the (possibly nested) leaf field is baked into
// the closure at MakeDecoder time, so Decode just iterates Ops without
// needing to know anything about struct shape or nesting.
type decOp func(root reflect.Value, s string) error

type Decoder struct {
	Ops []decOp
}

type DecoderT[T any] struct { // T is used for decode
	Decoder
}

type decoderMap map[reflect.Kind]decOp

var globalDecoders decoderMap = makeStandardDecoderMap(decoderMap{})

func makeStandardDecoderMap(m decoderMap) decoderMap {
	m[reflect.Bool] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetBool(false)
			return nil
		}
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		v.SetBool(b)
		return nil
	}
	m[reflect.Int] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetInt(0)
			return nil
		}
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(i)
		return nil
	}
	m[reflect.String] = func(v reflect.Value, s string) error {
		v.SetString(s)
		return nil
	}
	m[reflect.Float64] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetFloat(0)
			return nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		v.SetFloat(f)
		return nil
	}
	m[reflect.Float32] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetFloat(0)
			return nil
		}
		f, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return err
		}
		v.SetFloat(f)
		return nil
	}
	m[reflect.Int64] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetInt(0)
			return nil
		}
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(i)
		return nil
	}
	m[reflect.Int32] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetInt(0)
			return nil
		}
		i, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return err
		}
		v.SetInt(i)
		return nil
	}
	m[reflect.Int16] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetInt(0)
			return nil
		}
		i, err := strconv.ParseInt(s, 10, 16)
		if err != nil {
			return err
		}
		v.SetInt(i)
		return nil
	}
	m[reflect.Int8] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetInt(0)
			return nil
		}
		i, err := strconv.ParseInt(s, 10, 8)
		if err != nil {
			return err
		}
		v.SetInt(i)
		return nil
	}
	m[reflect.Uint] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetUint(0)
			return nil
		}
		i, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(i)
		return nil
	}
	m[reflect.Uint64] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetUint(0)
			return nil
		}
		i, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(i)
		return nil
	}
	m[reflect.Uint32] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetUint(0)
			return nil
		}
		i, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return err
		}
		v.SetUint(i)
		return nil
	}
	m[reflect.Uint16] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetUint(0)
			return nil
		}
		i, err := strconv.ParseUint(s, 10, 16)
		if err != nil {
			return err
		}
		v.SetUint(i)
		return nil
	}
	m[reflect.Uint8] = func(v reflect.Value, s string) error {
		if s == "" {
			v.SetUint(0)
			return nil
		}
		i, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			return err
		}
		v.SetUint(i)
		return nil
	}
	// m[reflect.Struct] = llok at  MakeDecoderAny

	return m
}

func MakeDecoder[T any](header []string) *DecoderT[T] {
	hmap := headerToMap(header)
	rtype := reflect.TypeFor[T]()

	return &DecoderT[T]{
		Decoder: MakeDecoderAny(hmap, rtype),
	}
}

type CsvParser interface {
	ParseToAny(string) any
}

// Builds a transformer from header defined row to rtype struct
func MakeDecoderAny(header headerMap, rtype reflect.Type) Decoder {
	var d Decoder
	appendDecodeOps(&d, header, rtype, nil)
	return d
}

// appendDecodeOps walks header/rtype, appending one decOp per leaf (scalar
// or CsvParser) field to d.Ops. path is the field-index path, relative to
// the root struct, of rtype itself, so ops built for nested structs bake in
// the full path down from the root rather than just their local field index.
func appendDecodeOps(d *Decoder, header headerMap, rtype reflect.Type, path []int) {
	fields := fieldsByName(rtype)

	for name, header := range header.All() {
		name = strcase.ToCamel(name)
		field, ok := fields[name]
		if !ok {
			continue // TODO: handle error
		}
		fieldPath := joinIndex(path, field.Index)

		if header, ok := header.(headerMap); ok {
			if field.Type.Kind() != reflect.Struct {
				continue // TODO: handle error
			}
			appendDecodeOps(d, header, field.Type, fieldPath)
			continue
		}

		parserType := reflect.TypeFor[CsvParser]()
		if field.Type.Implements(parserType) {
			d.Ops = append(d.Ops, func(root reflect.Value, s string) error {
				v := root.FieldByIndex(fieldPath)
				method := gx.MustValid(v.MethodByName("ParseToAny"))
				pv := method.Call([]reflect.Value{
					reflect.ValueOf(s),
				})
				v.Set(pv[0].Elem())

				return nil
			})
			continue
		}

		if op, ok := globalDecoders[field.Type.Kind()]; ok {
			d.Ops = append(d.Ops, func(root reflect.Value, s string) error {
				return op(root.FieldByIndex(fieldPath), s)
			})
			continue
		}

		log.Fatal("wht")
	}
}

func (d DecoderT[T]) Decode(row []string) (T, error) {
	var item T
	v := reflect.ValueOf(&item).Elem()

	var firstErr error
	for i, op := range d.Ops {
		if i >= len(row) {
			continue
		}
		if err := op(v, row[i]); err != nil && firstErr == nil {
			firstErr = err // skip faulty field, leave zero value
		}
	}

	return item, firstErr
}
