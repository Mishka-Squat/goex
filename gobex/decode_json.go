package gobex

import (
	"errors"
	"reflect"
)

func decAnyValue(state *decoderState, kind reflect.Kind) any {
	// Index by Go types.
	switch kind {
	case reflect.Bool:
		return must(decBoolValue(state))
	case reflect.Int8:
		return must(decInt8Value(state))
	case reflect.Int16:
		return must(decInt16Value(state))
	case reflect.Int32:
		return must(decInt32Value(state))
	case reflect.Int64:
		return must(decInt64Value(state))
	case reflect.Uint8:
		return must(decUint8Value(state))
	case reflect.Uint16:
		return must(decUint16Value(state))
	case reflect.Uint32:
		return must(decUint32Value(state))
	case reflect.Uint64:
		return must(decUint64Value(state))
	case reflect.Float32:
		return must(decFloat32Value(state))
	case reflect.Float64:
		return must(decFloat64Value(state))
	case reflect.Complex64:
		return must(decComplex64Value(state))
	case reflect.Complex128:
		return must(decComplex128Value(state))
	case reflect.String:
		return decStringValue(state)
	}

	return nil
}

// decodeSlice decodes a slice and stores it in value.
// Slices are encoded as an unsigned length followed by the elements.
func (dec *Decoder) decodeSliceAny(state *decoderState, elemId typeId) []any {
	u := state.decodeUint()

	var kind reflect.Kind
	size := state.dec.typeSize(elemId)

	nBytes := u * uint64(size)
	n := int(u)
	// Take care with overflow in this calculation.
	if n < 0 || uint64(n) != u || nBytes > tooBig || (size > 0 && nBytes/uint64(size) != u) {
		// We don't check n against buffer length here because if it's a slice
		// of interfaces, there will be buffer reloads.
		errorf("%d slice too big: %d elements of %d bytes", elemId, u, size)
	}
	value := make([]any, u)
	if elemId.isBuiltin() {
		kind = dec.typeKind(elemId)
		for i := range value {
			value[i] = decAnyValue(state, kind)
		}
	} else {
		for i := range value {
			value[i] = dec.decodeJsonAnyValue(elemId)
		}
	}
	//dec.decodeArrayHelper(state, value, elemOp, n, ovfl, helper)

	return value
}

// decodeJsonMapValue decodes the data stream representing a value and stores it in value.
func (dec *Decoder) decodeJsonAnyValue(wireId typeId) any {
	wt := dec.wireType[wireId.id()]

	if wt.StructT != nil {
		return dec.decodeJsonStruct(wt)
	}

	state := dec.newDecoderState(&dec.buf)
	state.fieldnum = singletonField
	if state.decodeUint() != 0 {
		errorf("decode: corrupted data: non-zero delta for singleton")
	}
	if wt.ArrayT != nil || wt.SliceT != nil {
		return dec.decodeJsonSlice(&state, wt)
	}

	return nil
}

var errNoError error = errors.New("no error")

// decodeJsonStruct decodes the data stream representing a value and stores it in value.
func (dec *Decoder) decodeJsonStruct(wt *wireType) map[string]any {
	if wt.StructT == nil {
		return nil
	}

	defer catchError(&dec.err)
	state := dec.newDecoderState(&dec.buf)
	state.fieldnum = -1

	value := map[string]any{}
	fields := wt.StructT.Field
	for state.b.Len() > 0 {
		delta := int(state.decodeUint())
		if delta < 0 {
			errorf("decode: corrupted data: negative delta")
		}
		if delta == 0 { // struct terminator is zero delta fieldnum
			break
		}
		if state.fieldnum >= len(fields)-delta { // subtract to compare without overflow
			error_(errRange)
		}
		fieldnum := state.fieldnum + delta
		field := &fields[fieldnum]
		// assume instr.index is always equal to fieldIndex
		//if instr.index != nil {
		//	// Otherwise the field is unknown to us and instr.op is an ignore op.
		//	field = value.FieldByIndex(instr.index)
		//	if field.Kind() == reflect.Pointer {
		//		field = decAlloc(field)
		//	}
		//}

		switch kind := dec.typeKind(field.Id); kind {
		case reflect.Array:
			//state.dec.decodeArray(state, value, *elemOp, t.Len(), ovfl, helper)
		case reflect.Map:
			//state.dec.decodeMap(t, state, value, *keyOp, *elemOp, ovfl)
		case reflect.Slice:
			var elemId typeId
			if tt := builtinIdToType(field.Id); tt != nil {
				elemId = tt.(*sliceType).Elem
			} else {
				elemId = dec.wireType[field.Id.id()].SliceT.Elem
			}
			value[field.Name] = state.dec.decodeSliceAny(&state, elemId /*, *elemOp, ovfl, helper*/)
		case reflect.Struct:
			value[field.Name] = dec.decodeJsonStruct(dec.wireType[field.Id.id()])
		case reflect.Interface:
			//state.dec.decodeInterface(t, state, value)
		default:
			value[field.Name] = decAnyValue(&state, kind)
		}
		//var field reflect.Value
		//instr.op(instr, state, field)
		state.fieldnum = fieldnum
	}

	return value
}

// decodeJsonSliceValue decodes the data stream representing a value and stores it in value.
func (dec *Decoder) decodeJsonSlice(state *decoderState, wt *wireType) []any {
	defer catchError(&dec.err)

	var value []any
	if t := wt.ArrayT; t != nil {
		value = make([]any, t.Len)
		//value[f.Name] = nil
	} else if t := wt.SliceT; t != nil {
		value = state.dec.decodeSliceAny(state, t.Elem /*, *elemOp, ovfl, helper*/)
	}

	return value
}
