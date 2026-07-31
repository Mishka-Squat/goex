package csvex

import (
	"reflect"
	"strconv"

	"github.com/Mishka-Squat/mengine/src/core/log"
	"github.com/iancoleman/strcase"
)

// encOp always receives the *root* struct value (T). The reflect
// field-index path down to the (possibly nested) leaf field is baked into
// the closure at MakeEncoder time, so Encode just iterates Ops without
// needing to know anything about struct shape or nesting.
type encOp func(root reflect.Value) (string, error)

type Encoder struct {
	Ops []encOp
}

type EncoderT[T any] struct { // T is used for encode
	Encoder
}

type CsvFormatter interface {
	String() string
}

type encoderMap map[reflect.Kind]encOp

var globalEncoders encoderMap = makeStandardEncoderMap(encoderMap{})

func makeStandardEncoderMap(m encoderMap) encoderMap {
	m[reflect.Bool] = func(v reflect.Value) (string, error) {
		return strconv.FormatBool(v.Bool()), nil
	}
	m[reflect.Int] = func(v reflect.Value) (string, error) {
		return strconv.FormatInt(v.Int(), 10), nil
	}
	m[reflect.String] = func(v reflect.Value) (string, error) {
		return v.String(), nil
	}
	m[reflect.Float64] = func(v reflect.Value) (string, error) {
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil
	}
	m[reflect.Float32] = func(v reflect.Value) (string, error) {
		return strconv.FormatFloat(v.Float(), 'f', -1, 32), nil
	}
	m[reflect.Int64] = func(v reflect.Value) (string, error) {
		return strconv.FormatInt(v.Int(), 10), nil
	}
	m[reflect.Int32] = func(v reflect.Value) (string, error) {
		return strconv.FormatInt(v.Int(), 10), nil
	}
	m[reflect.Int16] = func(v reflect.Value) (string, error) {
		return strconv.FormatInt(v.Int(), 10), nil
	}
	m[reflect.Int8] = func(v reflect.Value) (string, error) {
		return strconv.FormatInt(v.Int(), 10), nil
	}
	m[reflect.Uint] = func(v reflect.Value) (string, error) {
		return strconv.FormatUint(v.Uint(), 10), nil
	}
	m[reflect.Uint64] = func(v reflect.Value) (string, error) {
		return strconv.FormatUint(v.Uint(), 10), nil
	}
	m[reflect.Uint32] = func(v reflect.Value) (string, error) {
		return strconv.FormatUint(v.Uint(), 10), nil
	}
	m[reflect.Uint16] = func(v reflect.Value) (string, error) {
		return strconv.FormatUint(v.Uint(), 10), nil
	}
	m[reflect.Uint8] = func(v reflect.Value) (string, error) {
		return strconv.FormatUint(v.Uint(), 10), nil
	}

	return m
}

func MakeEncoder[T any](header []string) *EncoderT[T] {
	hmap := headerToMap(header)
	rtype := reflect.TypeFor[T]()

	return &EncoderT[T]{
		Encoder: MakeEncoderAny(rtype, hmap),
	}
}

// Builds a transformer from rtype struct to header difned row
func MakeEncoderAny(rtype reflect.Type, header headerMap) Encoder {
	var e Encoder
	e.appendEncodeOps(rtype, header)
	return e
}

// appendEncodeOps walks rtype/header, appending one encOp per leaf (scalar
// or CsvFormatter) field to e.Ops. path is the field-index path, relative to
// the root struct, of rtype itself, so ops built for nested structs bake in
// the full path down from the root rather than just their local field index.
func (e *Encoder) appendEncodeOps(rtype reflect.Type, header headerMap, path ...int) {
	fields := fieldsByName(rtype)

	for name, header := range header.All() {
		name = strcase.ToCamel(name)
		field, ok := fields[name]
		if !ok {
			continue // TODO: handle error
		}
		fieldPath := joinIndex(path, field.Index)

		if header, ok := header.(headerMap); ok {
			switch field.Type.Kind() {
			case reflect.Struct:
				e.appendEncodeOps(field.Type, header, fieldPath...)
			case reflect.Map:
				e.appendMapEncodeOps(field.Type, header, fieldPath)
			default:
				// TODO: handle error
			}
			continue
		}

		formatterType := reflect.TypeFor[CsvFormatter]()
		if field.Type.Implements(formatterType) {
			e.Ops = append(e.Ops, func(root reflect.Value) (string, error) {
				v := root.FieldByIndex(fieldPath)
				return v.Interface().(CsvFormatter).String(), nil
			})
			continue
		}

		if op, ok := globalEncoders[field.Type.Kind()]; ok {
			e.Ops = append(e.Ops, func(root reflect.Value) (string, error) {
				return op(root.FieldByIndex(fieldPath))
			})
			continue
		}

		log.Fatal("wht")
	}
}

// appendMapEncodeOps appends one encOp per header entry for a map-typed
// field, e.g. "yield.Food"/"yield.Ore" for a map[resourceKind]uint8 field.
// The header segment after the dot ("Food") is parsed straight into a map
// key via the key type's CsvParser, mirroring how a struct's .Field name is
// resolved for scalar/nested leaves.
func (e *Encoder) appendMapEncodeOps(mapType reflect.Type, header headerMap, fieldPath []int) {
	keyType := mapType.Key()
	elemType := mapType.Elem()

	parserType := reflect.TypeFor[CsvParser]()
	if !reflect.PointerTo(keyType).Implements(parserType) {
		log.Fatal("wht")
	}

	formatterType := reflect.TypeFor[CsvFormatter]()

	for name, leaf := range header.All() {
		if _, ok := leaf.(headerMap); ok {
			continue // TODO: handle error, nested maps not supported
		}

		mapKey := reflect.New(keyType).Elem()
		if err := mapKey.Addr().Interface().(CsvParser).Parse(name); err != nil {
			log.Fatal("wht")
		}

		if elemType.Implements(formatterType) {
			e.Ops = append(e.Ops, func(root reflect.Value) (string, error) {
				m := root.FieldByIndex(fieldPath)
				v := m.MapIndex(mapKey)
				if !v.IsValid() {
					return "", nil
				}
				return v.Interface().(CsvFormatter).String(), nil
			})
			continue
		}

		if op, ok := globalEncoders[elemType.Kind()]; ok {
			e.Ops = append(e.Ops, func(root reflect.Value) (string, error) {
				m := root.FieldByIndex(fieldPath)
				v := m.MapIndex(mapKey)
				if !v.IsValid() {
					return "", nil
				}
				return op(v)
			})
			continue
		}

		log.Fatal("wht")
	}
}

func (e EncoderT[T]) Encode(item T, row []string) error {
	v := reflect.ValueOf(item)

	var firstErr error
	for i, op := range e.Ops {
		s, err := op(v)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue // skip faulty field, leave zero value
		}
		row[i] = s
	}

	return firstErr
}
