package gobex

import "reflect"

func decAnyValue(kind reflect.Kind, i *decInstr, state *decoderState) any {
	// Index by Go types.
	switch kind {
	case reflect.Bool:
		return decBoolValue(i, state)
	case reflect.Int8:
		return decInt8Value(i, state)
	case reflect.Int16:
		return decInt16Value(i, state)
	case reflect.Int32:
		return decInt32Value(i, state)
	case reflect.Int64:
		return decInt64Value(i, state)
	case reflect.Uint8:
		return decUint8Value(i, state)
	case reflect.Uint16:
		return decUint16Value(i, state)
	case reflect.Uint32:
		return decUint32Value(i, state)
	case reflect.Uint64:
		return decUint64Value(i, state)
	case reflect.Float32:
		return decFloat32Value(i, state)
	case reflect.Float64:
		return decFloat64Value(i, state)
	case reflect.Complex64:
		return decComplex64Value(i, state)
	case reflect.Complex128:
		return decComplex128Value(i, state)
	case reflect.String:
		return decStringValue(i, state)
	}

	return nil
}

// decodeSlice decodes a slice and stores it in value.
// Slices are encoded as an unsigned length followed by the elements.
func (dec *Decoder) decodeSliceAny(state *decoderState, elemId typeId /*, elemOp decOp, ovfl error, helper decHelper*/) []any {
	u := state.decodeUint()
	size := 0

	if elemId.isBuiltin() {
		bt := builtinIdToType(elemId)
		size = bt.size()
	} else {

	}

	nBytes := int(u) * size
	n := int(u)
	// Take care with overflow in this calculation.
	if n < 0 || uint64(n) != u || nBytes > tooBig || (size > 0 && uint64(nBytes/size) != u) {
		// We don't check n against buffer length here because if it's a slice
		// of interfaces, there will be buffer reloads.
		errorf("%d slice too big: %d elements of %d bytes", elemId, u, size)
	}
	value := make([]any, u)
	if elemId.isBuiltin() {
		for i := range value {
			_ = i
			//value[i] = decAnyValue(reflect.String, )
		}
	}
	//dec.decodeArrayHelper(state, value, elemOp, n, ovfl, helper)

	return value
}

// decodeJsonMapValue decodes the data stream representing a value and stores it in value.
func (dec *Decoder) decodeJsonAnyValue(wireId typeId) any {
	wt := dec.wireType[wireId]

	if wt.StructT != nil {
		return dec.decodeJsonMapValue(wt)
	}
	if wt.ArrayT != nil || wt.SliceT != nil {
		return dec.decodeJsonSliceValue(wt)
	}

	return nil
}

func (dec *Decoder) decodeJsonMapStruct(value map[string]any, fields []fieldType) {
	state := dec.newDecoderState(&dec.buf)
	defer dec.freeDecoderState(state)
	state.fieldnum = -1
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

		fieldtype := dec.wireType[field.Id]
		switch fieldtype.Kind() {
		case reflect.Array:
			//state.dec.decodeArray(state, value, *elemOp, t.Len(), ovfl, helper)
		case reflect.Map:
			//state.dec.decodeMap(t, state, value, *keyOp, *elemOp, ovfl)
		case reflect.Slice:
			//if tt := builtinIdToType(wireId); tt != nil {
			//	elemId = tt.(*sliceType).Elem
			//} else {
			value[field.Name] = state.dec.decodeSliceAny(state, fieldtype.SliceT.Elem /*, *elemOp, ovfl, helper*/)
		case reflect.Struct:
			//dec.decodeStruct(*enginePtr, value)
		case reflect.Interface:
			//state.dec.decodeInterface(t, state, value)
		}
		//var field reflect.Value
		//instr.op(instr, state, field)
		state.fieldnum = fieldnum
	}
}

// decodeJsonMapValue decodes the data stream representing a value and stores it in value.
func (dec *Decoder) decodeJsonMapValue(wt *wireType) map[string]any {
	defer catchError(&dec.err)

	value := map[string]any{}
	if st := wt.StructT; st != nil {
		dec.decodeJsonMapStruct(value, st.Field)
		//for _, f := range st.Field {
		//	value[f.Name] = dec.decodeJsonAnyValue(f.Id)
		//}
	}
	return value
}

// decodeJsonSliceValue decodes the data stream representing a value and stores it in value.
func (dec *Decoder) decodeJsonSliceValue(wt *wireType) []any {
	defer catchError(&dec.err)

	var value []any
	if t := wt.ArrayT; t != nil {
		value = make([]any, t.Len)
		//value[f.Name] = nil
	} else if t := wt.SliceT; t != nil {
		//value = make([]any, t.)
	}

	return value
}
