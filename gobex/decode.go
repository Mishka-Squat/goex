// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run decgen.go -output dec_helpers.go

package gobex

import (
	"encoding"
	"errors"
	"io"
	"math"
	"math/bits"
	"reflect"

	"github.com/Mishka-Squat/goex/gx"
)

var (
	errBadUint  = errors.New("gob: encoded unsigned integer out of range")
	errBadType  = errors.New("gob: unknown type id or corrupted data")
	errRange    = errors.New("gob: bad data: field numbers out of bounds")
	errOverflow = errors.New("gob: value overflow")
)

type decHelper func(state *decoderState, v reflect.Value, length int) error

// decoderState is the execution state of an instance of the decoder. A new state
// is created for nested objects.
type decoderState struct {
	dec *Decoder
	// The buffer is stored with an extra indirection because it may be replaced
	// if we load a type during decode (when reading an interface value).
	b        *decBuffer
	fieldnum int           // the last field number read.
	next     *decoderState // for free list
}

// decBuffer is an extremely simple, fast implementation of a read-only byte buffer.
// It is initialized by calling Size and then copying the data into the slice returned by Bytes().
type decBuffer struct {
	data   []byte
	offset int // Read offset.
}

func (d *decBuffer) Read(p []byte) (int, error) {
	n := copy(p, d.data[d.offset:])
	if n == 0 && len(p) != 0 {
		return 0, io.EOF
	}
	d.offset += n
	return n, nil
}

func (d *decBuffer) Drop(n int) {
	if n > d.Len() {
		panic("drop")
	}
	d.offset += n
}

func (d *decBuffer) ReadByte() (byte, error) {
	if d.offset >= len(d.data) {
		return 0, io.EOF
	}
	c := d.data[d.offset]
	d.offset++
	return c, nil
}

func (d *decBuffer) Len() int {
	return len(d.data) - d.offset
}

func (d *decBuffer) Bytes() []byte {
	return d.data[d.offset:]
}

// SetBytes sets the buffer to the bytes, discarding any existing data.
func (d *decBuffer) SetBytes(data []byte) {
	d.data = data
	d.offset = 0
}

func (d *decBuffer) Reset() {
	d.data = d.data[0:0]
	d.offset = 0
}

// We pass the bytes.Buffer separately for easier testing of the infrastructure
// without requiring a full Decoder.
func (dec *Decoder) newDecoderState(buf *decBuffer) decoderState {
	return decoderState{
		dec: dec,
		b:   buf,
	}
}

func overflow(name string) error {
	return errors.New(`value for "` + name + `" out of range`)
}

// decodeUintReader reads an encoded unsigned integer from an io.Reader.
// Used only by the Decoder to read the message length.
func decodeUintReader(r io.Reader, buf []byte) (x uint64, width int, err error) {
	width = 1
	n, err := io.ReadFull(r, buf[0:width])
	if n == 0 {
		return
	}
	b := buf[0]
	if b <= 0x7f {
		return uint64(b), width, nil
	}
	n = -int(int8(b))
	if n > uint64Size {
		err = errBadUint
		return
	}
	width, err = io.ReadFull(r, buf[0:n])
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return
	}
	// Could check that the high byte is zero but it's not worth it.
	for _, b := range buf[0:width] {
		x = x<<8 | uint64(b)
	}
	width++ // +1 for length byte
	return
}

// decodeUint reads an encoded unsigned integer from state.r.
// Does not check for overflow.
func (state *decoderState) decodeUint() (x uint64) {
	b, err := state.b.ReadByte()
	if err != nil {
		error_(err)
	}
	if b <= 0x7f {
		return uint64(b)
	}
	n := -int(int8(b))
	if n > uint64Size {
		error_(errBadUint)
	}
	buf := state.b.Bytes()
	if len(buf) < n {
		errorf("invalid uint data length %d: exceeds input size %d", n, len(buf))
	}
	// Don't need to check error; it's safe to loop regardless.
	// Could check that the high byte is zero but it's not worth it.
	for _, b := range buf[0:n] {
		x = x<<8 | uint64(b)
	}
	state.b.Drop(n)
	return x
}

// decodeInt reads an encoded signed integer from state.r.
// Does not check for overflow.
func (state *decoderState) decodeInt() int64 {
	x := state.decodeUint()
	if x&1 != 0 {
		return ^int64(x >> 1)
	}
	return int64(x >> 1)
}

// getLength decodes the next uint and makes sure it is a possible
// size for a data item that follows, which means it must fit in a
// non-negative int and fit in the buffer.
func (state *decoderState) getLength() (int, bool) {
	n := int(state.decodeUint())
	if n < 0 || state.b.Len() < n || tooBig <= n {
		return 0, false
	}
	return n, true
}

// decOp is the signature of a decoding operator for a given type.
type decOp func(state *decoderState, v reflect.Value)

// The 'instructions' of the decoding machine
type decInstr struct {
	op    decOp
	field int   // field number of the wire type
	index []int // field access indices for destination type
}

// ignoreUint discards a uint value with no destination.
func ignoreUint(state *decoderState, v reflect.Value) {
	state.decodeUint()
}

// ignoreTwoUints discards a uint value with no destination. It's used to skip
// complex values.
func ignoreTwoUints(state *decoderState, v reflect.Value) {
	state.decodeUint()
	state.decodeUint()
}

// Since the encoder writes no zeros, if we arrive at a decoder we have
// a value to extract and store. The field number has already been read
// (it's how we knew to call this decoder).
// Each decoder is responsible for handling any indirections associated
// with the data structure. If any pointer so reached is nil, allocation must
// be done.

// decAlloc takes a value and returns a settable value that can
// be assigned to. If the value is a pointer, decAlloc guarantees it points to storage.
// The callers to the individual decoders are expected to have used decAlloc.
// The individual decoders don't need it.
func decAlloc(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v
}

// decBoolValue decodes a uint and stores it as a boolean in value.
func decBoolValue(state *decoderState) (bool, error) {
	return state.decodeUint() != 0, nil
}

// decInt8Value decodes an integer and stores it as an int8 in value.
func decInt8Value(state *decoderState) (int8, error) {
	v := state.decodeInt()
	if v < math.MinInt8 || math.MaxInt8 < v {
		return 0, errOverflow
	}
	return int8(v), nil
}

// decUint8Value decodes an unsigned integer and stores it as a uint8 in value.
func decUint8Value(state *decoderState) (uint8, error) {
	v := state.decodeUint()
	if math.MaxUint8 < v {
		return 0, errOverflow
	}
	return uint8(v), nil
}

// decInt16Value decodes an integer and stores it as an int16 in value.
func decInt16Value(state *decoderState) (int16, error) {
	v := state.decodeInt()
	if v < math.MinInt16 || math.MaxInt16 < v {
		return 0, errOverflow
	}
	return int16(v), nil
}

// decUint16Value decodes an unsigned integer and stores it as a uint16 in value.
func decUint16Value(state *decoderState) (uint16, error) {
	v := state.decodeUint()
	if math.MaxUint16 < v {
		return 0, errOverflow
	}
	return uint16(v), nil
}

// decInt32Value decodes an integer and stores it as an int32 in value.
func decInt32Value(state *decoderState) (int32, error) {
	v := state.decodeInt()
	if v < math.MinInt32 || math.MaxInt32 < v {
		return 0, errOverflow
	}
	return int32(v), nil
}

// decUint32Value decodes an unsigned integer and stores it as a uint32 in value.
func decUint32Value(state *decoderState) (uint32, error) {
	v := state.decodeUint()
	if math.MaxUint32 < v {
		return 0, errOverflow
	}
	return uint32(v), nil
}

// decInt64Value decodes an integer and stores it as an int64 in value.
func decInt64Value(state *decoderState) (int64, error) {
	return state.decodeInt(), nil
}

// decUint64 decodes an unsigned integer and stores it as an uint64 in value.
func decUint64Value(state *decoderState) (uint64, error) {
	return state.decodeUint(), nil
}

// decFloat32Value decodes an unsigned integer, treats it as a 32-bit floating-point
// number, and stores it in value.
func decFloat32Value(state *decoderState) (float32, error) {
	return float32FromBits(state.decodeUint())
}

// decFloat64Value decodes an unsigned integer, treats it as a 64-bit floating-point
// number, and stores it in value.
func decFloat64Value(state *decoderState) (float64, error) {
	return float64FromBits(state.decodeUint())
}

// decComplex64Value decodes a pair of unsigned integers, treats them as a
// pair of floating point numbers, and stores them as a complex64 in value.
// The real part comes first.
func decComplex64Value(state *decoderState) (complex64, error) {
	real, err := float32FromBits(state.decodeUint())
	if err != nil {
		return complex(0, 0), err
	}

	imag, err := float32FromBits(state.decodeUint())
	if err != nil {
		return complex(0, 0), err
	}

	return complex(real, imag), nil
}

// decComplex128Value decodes a pair of unsigned integers, treats them as a
// pair of floating point numbers, and stores them as a complex128 in value.
// The real part comes first.
func decComplex128Value(state *decoderState) (complex128, error) {
	real, err := float64FromBits(state.decodeUint())
	if err != nil {
		return complex(0, 0), err
	}
	imag, err := float64FromBits(state.decodeUint())
	if err != nil {
		return complex(0, 0), err
	}
	return complex(real, imag), nil
}

// decStringValue decodes byte array and stores in value a string header
// describing the data.
// Strings are encoded as an unsigned count followed by the raw bytes.
func decStringValue(state *decoderState) string {
	n, ok := state.getLength()
	if !ok {
		errorf("bad %s slice length: %d", "[]string", n)
	}
	// Read the data.
	data := state.b.Bytes()
	if len(data) < n {
		errorf("invalid string length %d: exceeds input size %d", n, len(data))
	}
	s := string(data[:n])
	state.b.Drop(n)
	return s
}

// decBool decodes a uint and stores it as a boolean in value.
func decBool(state *decoderState, value reflect.Value) {
	value.SetBool(must(decBoolValue(state)))
}

// decInt8 decodes an integer and stores it as an int8 in value.
func decInt8(state *decoderState, value reflect.Value) {
	value.SetInt(int64(must(decInt8Value(state))))
}

// decUint8 decodes an unsigned integer and stores it as a uint8 in value.
func decUint8(state *decoderState, value reflect.Value) {
	value.SetUint(uint64(must(decUint8Value(state))))
}

// decInt16 decodes an integer and stores it as an int16 in value.
func decInt16(state *decoderState, value reflect.Value) {
	value.SetInt(int64(must(decInt16Value(state))))
}

// decUint16 decodes an unsigned integer and stores it as a uint16 in value.
func decUint16(state *decoderState, value reflect.Value) {
	value.SetUint(uint64(must(decUint16Value(state))))
}

// decInt32 decodes an integer and stores it as an int32 in value.
func decInt32(state *decoderState, value reflect.Value) {
	value.SetInt(int64(must(decInt32Value(state))))
}

// decUint32 decodes an unsigned integer and stores it as a uint32 in value.
func decUint32(state *decoderState, value reflect.Value) {
	value.SetUint(uint64(must(decUint32Value(state))))
}

// decInt64 decodes an integer and stores it as an int64 in value.
func decInt64(state *decoderState, value reflect.Value) {
	value.SetInt(must(decInt64Value(state)))
}

// decUint64 decodes an unsigned integer and stores it as a uint64 in value.
func decUint64(state *decoderState, value reflect.Value) {
	value.SetUint(must(decUint64Value(state)))
}

// Floating-point numbers are transmitted as uint64s holding the bits
// of the underlying representation. They are sent byte-reversed, with
// the exponent end coming out first, so integer floating point numbers
// (for example) transmit more compactly. This routine does the
// unswizzling.
func float64FromBits(u uint64) (float64, error) {
	v := bits.ReverseBytes64(u)
	return math.Float64frombits(v), nil
}

// float32FromBits decodes an unsigned integer, treats it as a 32-bit floating-point
// number, and returns it. It's a helper function for float32 and complex64.
// It returns a float64 because that's what reflection needs, but its return
// value is known to be accurately representable in a float32.
func float32FromBits(u uint64) (float32, error) {
	v, err := float64FromBits(u)
	if err != nil {
		return 0, err
	}

	av := v
	if av < 0 {
		av = -av
	}
	// +Inf is OK in both 32- and 64-bit floats. Underflow is always OK.
	if math.MaxFloat32 < av && av <= math.MaxFloat64 {
		return 0, errOverflow
	}
	return float32(v), nil
}

// decFloat32 decodes an unsigned integer, treats it as a 32-bit floating-point
// number, and stores it in value.
func decFloat32(state *decoderState, value reflect.Value) {
	value.SetFloat(float64(must(decFloat32Value(state))))
}

// decFloat64 decodes an unsigned integer, treats it as a 64-bit floating-point
// number, and stores it in value.
func decFloat64(state *decoderState, value reflect.Value) {
	value.SetFloat(must(decFloat64Value(state)))
}

// decComplex64 decodes a pair of unsigned integers, treats them as a
// pair of floating point numbers, and stores them as a complex64 in value.
// The real part comes first.
func decComplex64(state *decoderState, value reflect.Value) {
	value.SetComplex(complex128(must(decComplex64Value(state))))
}

// decComplex128 decodes a pair of unsigned integers, treats them as a
// pair of floating point numbers, and stores them as a complex128 in value.
// The real part comes first.
func decComplex128(state *decoderState, value reflect.Value) {
	value.SetComplex(must(decComplex128Value(state)))
}

// decUint8Slice decodes a byte slice and stores in value a slice header
// describing the data.
// uint8 slices are encoded as an unsigned count followed by the raw bytes.
func decUint8Slice(state *decoderState, value reflect.Value) {
	n, ok := state.getLength()
	if !ok {
		errorf("bad %s slice length: %d", value.Type(), n)
	}
	if value.Cap() < n {
		safe := saferio_SliceCap[byte](uint64(n))
		if safe < 0 {
			errorf("%s slice too big: %d elements", value.Type(), n)
		}
		value.Set(reflect.MakeSlice(value.Type(), safe, safe))
		ln := safe
		i := 0
		for i < n {
			if i >= ln {
				// We didn't allocate the entire slice,
				// due to using saferio.SliceCap.
				// Grow the slice for one more element.
				// The slice is full, so this should
				// bump up the capacity.
				value.Grow(1)
			}
			// Copy into s up to the capacity or n,
			// whichever is less.
			ln = value.Cap()
			if ln > n {
				ln = n
			}
			value.SetLen(ln)
			sub := value.Slice(i, ln)
			if _, err := state.b.Read(sub.Bytes()); err != nil {
				errorf("error decoding []byte at %d: %s", i, err)
			}
			i = ln
		}
	} else {
		value.SetLen(n)
		if _, err := state.b.Read(value.Bytes()); err != nil {
			errorf("error decoding []byte: %s", err)
		}
	}
}

// decString decodes byte array and stores in value a string header
// describing the data.
// Strings are encoded as an unsigned count followed by the raw bytes.
func decString(state *decoderState, value reflect.Value) {
	value.SetString(decStringValue(state))
}

// ignoreUint8Array skips over the data for a byte slice value with no destination.
func ignoreUint8Array(state *decoderState, value reflect.Value) {
	n, ok := state.getLength()
	if !ok {
		errorf("slice length too large")
	}
	bn := state.b.Len()
	if bn < n {
		errorf("invalid slice length %d: exceeds input size %d", n, bn)
	}
	state.b.Drop(n)
}

// Execution engine

// The encoder engine is an array of instructions indexed by field number of the incoming
// decoder. It is executed with random access according to field number.
type decEngine struct {
	instr    []decInstr
	numInstr int // the number of active instructions
}

// compileSingle compiles the decoder engine for a non-struct top-level value, including
// GobDecoders.
func (engine *decEngine) compileSingle(dec *Decoder, remoteId typeId, ut *userTypeInfo) error {
	rt := ut.user
	engine.instr = make([]decInstr, 1) // one item
	name := rt.String()                // best we can do
	if !dec.compatibleType(rt, remoteId, make(map[reflect.Type]typeId)) {
		remoteType := dec.typeString(remoteId)
		// Common confusing case: local interface type, remote concrete type.
		if ut.base.Kind() == reflect.Interface && remoteId.id() != tInterface.id() {
			return errors.New("gob: local interface type " + name + " can only be decoded from remote interface type; received concrete type " + remoteType)
		}
		return errors.New("gob: decoding into local type " + name + ", received remote type " + remoteType)
	}
	op := dec.decOpFor(remoteId, rt, name, make(map[reflect.Type]*decOp))
	engine.instr[singletonField] = decInstr{*op, singletonField, nil}
	engine.numInstr = 1
	return nil
}

// compileIgnoreSingle compiles the decoder engine for a non-struct top-level value that will be discarded.
func (engine *decEngine) compileIgnoreSingle(dec *Decoder, remoteId typeId) {
	engine.instr = make([]decInstr, 1) // one item
	op := dec.decIgnoreOpFor(remoteId, make(map[typeId]*decOp))
	engine.instr[0] = decInstr{*op, 0, nil}
	engine.numInstr = 1
}

// compileDec compiles the decoder engine for a value. If the value is not a struct,
// it calls out to compileSingle.
func (engine *decEngine) compileDec(dec *Decoder, remoteId typeId, ut *userTypeInfo) (err error) {
	defer catchError(&err)
	rt := ut.base
	srt := rt
	if srt.Kind() != reflect.Struct || ut.externalDec != 0 {
		return engine.compileSingle(dec, remoteId, ut)
	}
	var wireStruct *structType
	// Builtin types can come from global pool; the rest must be defined by the decoder.
	// Also we know we're decoding a struct now, so the client must have sent one.
	if t := builtinIdToType(remoteId); t != nil {
		wireStruct, _ = t.(*structType)
	} else {
		wire := dec.wireType[remoteId.id()]
		if wire == nil {
			error_(errBadType)
		}
		wireStruct = wire.StructT
	}
	if wireStruct == nil {
		errorf("type mismatch in decoder: want struct type %s; got non-struct", rt)
	}
	engine.instr = make([]decInstr, len(wireStruct.Field))
	seen := make(map[reflect.Type]*decOp)
	// Loop over the fields of the wire type.
	for fieldnum := 0; fieldnum < len(wireStruct.Field); fieldnum++ {
		wireField := wireStruct.Field[fieldnum]
		if wireField.Name == "" {
			errorf("empty name for remote field of type %s", wireStruct.Name)
		}
		// Find the field of the local type with the same name.
		localField, present := srt.FieldByName(wireField.Name)
		// TODO(r): anonymous names
		if !present || !isExported(wireField.Name) {
			op := dec.decIgnoreOpFor(wireField.Id, make(map[typeId]*decOp))
			engine.instr[fieldnum] = decInstr{*op, fieldnum, nil}
			continue
		}
		if !dec.compatibleType(localField.Type, wireField.Id, make(map[reflect.Type]typeId)) {
			errorf("wrong type (%s) for received field %s.%s", localField.Type, wireStruct.Name, wireField.Name)
		}
		op := dec.decOpFor(wireField.Id, localField.Type, localField.Name, seen)
		engine.instr[fieldnum] = decInstr{*op, fieldnum, localField.Index}
		engine.numInstr++
	}
	return
}

// decodeSingle decodes a top-level value that is not a struct and stores it in value.
// Such values are preceded by a zero, making them have the memory layout of a
// struct field (although with an illegal field number).
func (dec *Decoder) decodeSingle(engine *decEngine, value reflect.Value) {
	state := dec.newDecoderState(&dec.buf)
	state.fieldnum = singletonField
	if state.decodeUint() != 0 {
		errorf("decode: corrupted data: non-zero delta for singleton")
	}
	instr := &engine.instr[singletonField]
	instr.op(&state, value)
}

// decodeStruct decodes a top-level struct and stores it in value.
// Indir is for the value, not the type. At the time of the call it may
// differ from ut.indir, which was computed when the engine was built.
// This state cannot arise for decodeSingle, which is called directly
// from the user's value, not from the innards of an engine.
func (dec *Decoder) decodeStruct(engine *decEngine, value reflect.Value) {
	state := dec.newDecoderState(&dec.buf)
	state.fieldnum = -1
	for state.b.Len() > 0 {
		delta := int(state.decodeUint())
		if delta < 0 {
			errorf("decode: corrupted data: negative delta")
		}
		if delta == 0 { // struct terminator is zero delta fieldnum
			break
		}
		if state.fieldnum >= len(engine.instr)-delta { // subtract to compare without overflow
			error_(errRange)
		}
		fieldnum := state.fieldnum + delta
		instr := &engine.instr[fieldnum]
		var field reflect.Value
		if instr.index != nil {
			// Otherwise the field is unknown to us and instr.op is an ignore op.
			field = value.FieldByIndex(instr.index)
			if field.Kind() == reflect.Pointer {
				field = decAlloc(field)
			}
		}
		instr.op(&state, field)
		state.fieldnum = fieldnum
	}
}

var noValue reflect.Value

// ignoreStruct discards the data for a struct with no destination.
func (dec *Decoder) ignoreStruct(engine *decEngine) {
	state := dec.newDecoderState(&dec.buf)
	state.fieldnum = -1
	for state.b.Len() > 0 {
		delta := int(state.decodeUint())
		if delta < 0 {
			errorf("ignore decode: corrupted data: negative delta")
		}
		if delta == 0 { // struct terminator is zero delta fieldnum
			break
		}
		fieldnum := state.fieldnum + delta
		if fieldnum >= len(engine.instr) {
			error_(errRange)
		}
		instr := &engine.instr[fieldnum]
		instr.op(&state, noValue)
		state.fieldnum = fieldnum
	}
}

// ignoreSingle discards the data for a top-level non-struct value with no
// destination. It's used when calling Decode with a nil value.
func (dec *Decoder) ignoreSingle(engine *decEngine) {
	state := dec.newDecoderState(&dec.buf)
	state.fieldnum = singletonField
	delta := int(state.decodeUint())
	if delta != 0 {
		errorf("decode: corrupted data: non-zero delta for singleton")
	}
	instr := &engine.instr[singletonField]
	instr.op(&state, noValue)
}

// decodeArrayHelper does the work for decoding arrays and slices.
func (dec *Decoder) decodeArrayHelper(state *decoderState, value reflect.Value, elemOp decOp, length int, helper decHelper) {
	if helper != nil && gx.Ok(helper(state, value, length)) {
		return
	}
	isPtr := value.Type().Elem().Kind() == reflect.Pointer
	ln := value.Len()
	for i := 0; i < length; i++ {
		if state.b.Len() == 0 {
			errorf("decoding array or slice: length exceeds input size (%d elements)", length)
		}
		if i >= ln {
			// This is a slice that we only partially allocated.
			// Grow it up to length.
			value.Grow(1)
			cp := value.Cap()
			if cp > length {
				cp = length
			}
			value.SetLen(cp)
			ln = cp
		}
		v := value.Index(i)
		if isPtr {
			v = decAlloc(v)
		}
		elemOp(state, v)
	}
}

// decodeArray decodes an array and stores it in value.
// The length is an unsigned integer preceding the elements. Even though the length is redundant
// (it's part of the type), it's a useful check and is included in the encoding.
func (dec *Decoder) decodeArray(state *decoderState, value reflect.Value, elemOp decOp, length int, helper decHelper) {
	if n := state.decodeUint(); n != uint64(length) {
		errorf("length mismatch in decodeArray")
	}
	dec.decodeArrayHelper(state, value, elemOp, length, helper)
}

// decodeIntoValue is a helper for map decoding.
func decodeIntoValue(state *decoderState, op decOp, isPtr bool, value reflect.Value, instr *decInstr) reflect.Value {
	v := value
	if isPtr {
		v = decAlloc(value)
	}

	op(state, v)
	return value
}

// decodeMap decodes a map and stores it in value.
// Maps are encoded as a length followed by key:value pairs.
// Because the internals of maps are not visible to us, we must
// use reflection rather than pointer magic.
func (dec *Decoder) decodeMap(mtyp reflect.Type, state *decoderState, value reflect.Value, keyOp, elemOp decOp) {
	n := int(state.decodeUint())
	if value.IsNil() {
		value.Set(reflect.MakeMapWithSize(mtyp, n))
	}
	keyIsPtr := mtyp.Key().Kind() == reflect.Pointer
	elemIsPtr := mtyp.Elem().Kind() == reflect.Pointer
	keyInstr := &decInstr{keyOp, 0, nil}
	elemInstr := &decInstr{elemOp, 0, nil}
	keyP := reflect.New(mtyp.Key())
	elemP := reflect.New(mtyp.Elem())
	for i := 0; i < n; i++ {
		key := decodeIntoValue(state, keyOp, keyIsPtr, keyP.Elem(), keyInstr)
		elem := decodeIntoValue(state, elemOp, elemIsPtr, elemP.Elem(), elemInstr)
		value.SetMapIndex(key, elem)
		keyP.Elem().SetZero()
		elemP.Elem().SetZero()
	}
}

// ignoreArrayHelper does the work for discarding arrays and slices.
func (dec *Decoder) ignoreArrayHelper(state *decoderState, elemOp decOp, length int) {
	for i := 0; i < length; i++ {
		if state.b.Len() == 0 {
			errorf("decoding array or slice: length exceeds input size (%d elements)", length)
		}
		elemOp(state, noValue)
	}
}

// ignoreArray discards the data for an array value with no destination.
func (dec *Decoder) ignoreArray(state *decoderState, elemOp decOp, length int) {
	if n := state.decodeUint(); n != uint64(length) {
		errorf("length mismatch in ignoreArray")
	}
	dec.ignoreArrayHelper(state, elemOp, length)
}

// ignoreMap discards the data for a map value with no destination.
func (dec *Decoder) ignoreMap(state *decoderState, keyOp, elemOp decOp) {
	n := int(state.decodeUint())
	for i := 0; i < n; i++ {
		keyOp(state, noValue)
		elemOp(state, noValue)
	}
}

// decodeSlice decodes a slice and stores it in value.
// Slices are encoded as an unsigned length followed by the elements.
func (dec *Decoder) decodeSlice(state *decoderState, value reflect.Value, elemOp decOp, helper decHelper) {
	u := state.decodeUint()
	typ := value.Type()
	size := uint64(typ.Elem().Size())
	nBytes := u * size
	n := int(u)
	// Take care with overflow in this calculation.
	if n < 0 || uint64(n) != u || nBytes > tooBig || (size > 0 && nBytes/size != u) {
		// We don't check n against buffer length here because if it's a slice
		// of interfaces, there will be buffer reloads.
		errorf("%s slice too big: %d elements of %d bytes", typ.Elem(), u, size)
	}
	if value.Cap() < n {
		safe := saferio_SliceCapWithSize(size, uint64(n))
		if safe < 0 {
			errorf("%s slice too big: %d elements of %d bytes", typ.Elem(), u, size)
		}
		value.Set(reflect.MakeSlice(typ, safe, safe))
	} else {
		value.SetLen(n)
	}
	dec.decodeArrayHelper(state, value, elemOp, n, helper)
}

// ignoreSlice skips over the data for a slice value with no destination.
func (dec *Decoder) ignoreSlice(state *decoderState, elemOp decOp) {
	dec.ignoreArrayHelper(state, elemOp, int(state.decodeUint()))
}

// decodeInterface decodes an interface value and stores it in value.
// Interfaces are encoded as the name of a concrete type followed by a value.
// If the name is empty, the value is nil and no value is sent.
func (dec *Decoder) decodeInterface(ityp reflect.Type, state *decoderState, value reflect.Value) {
	// Read the name of the concrete type.
	nr := state.decodeUint()
	if nr > 1<<31 { // zero is permissible for anonymous types
		errorf("invalid type name length %d", nr)
	}
	if nr > uint64(state.b.Len()) {
		errorf("invalid type name length %d: exceeds input size", nr)
	}
	n := int(nr)
	name := state.b.Bytes()[:n]
	state.b.Drop(n)
	// Allocate the destination interface value.
	if len(name) == 0 {
		// Copy the nil interface value to the target.
		value.SetZero()
		return
	}
	if len(name) > 1024 {
		errorf("name too long (%d bytes): %.20q...", len(name), name)
	}
	// The concrete type must be registered.
	typi, ok := nameToConcreteType.Load(string(name))
	if !ok {
		errorf("name not registered for interface: %q", name)
	}
	typ := typi.(reflect.Type)

	// Read the type id of the concrete value.
	concreteId := dec.decodeTypeSequence(true)
	if concreteId.id() < 0 {
		error_(dec.err)
	}
	// Byte count of value is next; we don't care what it is (it's there
	// in case we want to ignore the value by skipping it completely).
	state.decodeUint()
	// Read the concrete value.
	v := allocValue(typ)
	dec.decodeValue(concreteId, v)
	if dec.err != nil {
		error_(dec.err)
	}
	// Assign the concrete value to the interface.
	// Tread carefully; it might not satisfy the interface.
	if !typ.AssignableTo(ityp) {
		errorf("%s is not assignable to type %s", typ, ityp)
	}
	// Copy the interface value to the target.
	value.Set(v)
}

// ignoreInterface discards the data for an interface value with no destination.
func (dec *Decoder) ignoreInterface(state *decoderState) {
	// Read the name of the concrete type.
	n, ok := state.getLength()
	if !ok {
		errorf("bad interface encoding: name too large for buffer")
	}
	bn := state.b.Len()
	if bn < n {
		errorf("invalid interface value length %d: exceeds input size %d", n, bn)
	}
	state.b.Drop(n)
	id := dec.decodeTypeSequence(true)
	if id.id() < 0 {
		error_(dec.err)
	}
	// At this point, the decoder buffer contains a delimited value. Just toss it.
	n, ok = state.getLength()
	if !ok {
		errorf("bad interface encoding: data length too large for buffer")
	}
	state.b.Drop(n)
}

// decodeGobDecoder decodes something implementing the GobDecoder interface.
// The data is encoded as a byte slice.
func (dec *Decoder) decodeGobDecoder(ut *userTypeInfo, state *decoderState, value reflect.Value) {
	// Read the bytes for the value.
	n, ok := state.getLength()
	if !ok {
		errorf("GobDecoder: length too large for buffer")
	}
	b := state.b.Bytes()
	if len(b) < n {
		errorf("GobDecoder: invalid data length %d: exceeds input size %d", n, len(b))
	}
	b = b[:n]
	state.b.Drop(n)
	var err error
	// We know it's one of these.
	switch ut.externalDec {
	case xGob:
		gobDecoder, _ := reflect.TypeAssert[GobDecoder](value)
		err = gobDecoder.GobDecode(b)
	case xBinary:
		binaryUnmarshaler, _ := reflect.TypeAssert[encoding.BinaryUnmarshaler](value)
		err = binaryUnmarshaler.UnmarshalBinary(b)
	case xText:
		textUnmarshaler, _ := reflect.TypeAssert[encoding.TextUnmarshaler](value)
		err = textUnmarshaler.UnmarshalText(b)
	}
	if err != nil {
		error_(err)
	}
}

// ignoreGobDecoder discards the data for a GobDecoder value with no destination.
func (dec *Decoder) ignoreGobDecoder(state *decoderState) {
	// Read the bytes for the value.
	n, ok := state.getLength()
	if !ok {
		errorf("GobDecoder: length too large for buffer")
	}
	bn := state.b.Len()
	if bn < n {
		errorf("GobDecoder: invalid data length %d: exceeds input size %d", n, bn)
	}
	state.b.Drop(n)
}

// Index by Go types.
var decOpTable = [...]decOp{
	reflect.Bool:       decBool,
	reflect.Int8:       decInt8,
	reflect.Int16:      decInt16,
	reflect.Int32:      decInt32,
	reflect.Int64:      decInt64,
	reflect.Uint8:      decUint8,
	reflect.Uint16:     decUint16,
	reflect.Uint32:     decUint32,
	reflect.Uint64:     decUint64,
	reflect.Float32:    decFloat32,
	reflect.Float64:    decFloat64,
	reflect.Complex64:  decComplex64,
	reflect.Complex128: decComplex128,
	reflect.String:     decString,
}

// Indexed by gob types.  tComplex will be added during type.init().
var decIgnoreOpMap = map[pureTypeId]decOp{
	tBool.id():    ignoreUint,
	tInt.id():     ignoreUint,
	tUint.id():    ignoreUint,
	tFloat.id():   ignoreUint,
	tBytes.id():   ignoreUint8Array,
	tString.id():  ignoreUint8Array,
	tComplex.id(): ignoreTwoUints,
}

// decOpFor returns the decoding op for the base type under rt and
// the indirection count to reach it.
func (dec *Decoder) decOpFor(wireId typeId, rt reflect.Type, name string, inProgress map[reflect.Type]*decOp) *decOp {
	ut := userType(rt)
	// If the type implements GobEncoder, we handle it without further processing.
	if ut.externalDec != 0 {
		return dec.gobDecodeOpFor(ut)
	}

	// If this type is already in progress, it's a recursive type (e.g. map[string]*T).
	// Return the pointer to the op we're already building.
	if opPtr := inProgress[rt]; opPtr != nil {
		return opPtr
	}
	typ := ut.base
	var op decOp
	k := typ.Kind()
	if int(k) < len(decOpTable) {
		op = decOpTable[k]
	}
	if op == nil {
		inProgress[rt] = &op
		// Special cases
		switch t := typ; t.Kind() {
		case reflect.Array:
			name = "element of " + name
			elemId := dec.wireType[wireId.id()].ArrayT.Elem
			elemOp := dec.decOpFor(elemId, t.Elem(), name, inProgress)
			helper := decArrayHelper[t.Elem().Kind()]
			op = func(state *decoderState, value reflect.Value) {
				state.dec.decodeArray(state, value, *elemOp, t.Len(), helper)
			}

		case reflect.Map:
			keyId := dec.wireType[wireId.id()].MapT.Key
			elemId := dec.wireType[wireId.id()].MapT.Elem
			keyOp := dec.decOpFor(keyId, t.Key(), "key of "+name, inProgress)
			elemOp := dec.decOpFor(elemId, t.Elem(), "element of "+name, inProgress)
			op = func(state *decoderState, value reflect.Value) {
				state.dec.decodeMap(t, state, value, *keyOp, *elemOp)
			}

		case reflect.Slice:
			name = "element of " + name
			if t.Elem().Kind() == reflect.Uint8 {
				op = decUint8Slice
				break
			}
			var elemId typeId
			if tt := builtinIdToType(wireId); tt != nil {
				elemId = tt.(*sliceType).Elem
			} else {
				elemId = dec.wireType[wireId.id()].SliceT.Elem
			}
			elemOp := dec.decOpFor(elemId, t.Elem(), name, inProgress)
			helper := decSliceHelper[t.Elem().Kind()]
			op = func(state *decoderState, value reflect.Value) {
				state.dec.decodeSlice(state, value, *elemOp, helper)
			}

		case reflect.Struct:
			// Generate a closure that calls out to the engine for the nested type.
			ut := userType(typ)
			engine, err := dec.getDecEnginePtr(wireId, ut)
			if err != nil {
				error_(err)
			}
			op = func(state *decoderState, value reflect.Value) {
				// indirect through enginePtr to delay evaluation for recursive structs.
				dec.decodeStruct(engine, value)
			}
		case reflect.Interface:
			op = func(state *decoderState, value reflect.Value) {
				state.dec.decodeInterface(t, state, value)
			}
		}
	}
	if op == nil {
		errorf("decode can't handle type %s", rt)
	}
	return &op
}

var maxIgnoreNestingDepth = 10000

// decIgnoreOpFor returns the decoding op for a field that has no destination.
func (dec *Decoder) decIgnoreOpFor(wireId typeId, inProgress map[typeId]*decOp) *decOp {
	// Track how deep we've recursed trying to skip nested ignored fields.
	dec.ignoreDepth++
	defer func() { dec.ignoreDepth-- }()
	if dec.ignoreDepth > maxIgnoreNestingDepth {
		error_(errors.New("invalid nesting depth"))
	}
	// If this type is already in progress, it's a recursive type (e.g. map[string]*T).
	// Return the pointer to the op we're already building.
	if opPtr := inProgress[wireId]; opPtr != nil {
		return opPtr
	}
	op, ok := decIgnoreOpMap[wireId.id()]
	if !ok {
		inProgress[wireId] = &op
		if wireId.id() == tInterface.id() {
			// Special case because it's a method: the ignored item might
			// define types and we need to record their state in the decoder.
			op = func(state *decoderState, value reflect.Value) {
				state.dec.ignoreInterface(state)
			}
			return &op
		}
		// Special cases
		wire := dec.wireType[wireId.id()]
		switch {
		case wire == nil:
			errorf("bad data: undefined type %s", wireId.string())
		case wire.ArrayT != nil:
			elemId := wire.ArrayT.Elem
			elemOp := dec.decIgnoreOpFor(elemId, inProgress)
			op = func(state *decoderState, value reflect.Value) {
				state.dec.ignoreArray(state, *elemOp, wire.ArrayT.Len)
			}

		case wire.MapT != nil:
			keyId := dec.wireType[wireId.id()].MapT.Key
			elemId := dec.wireType[wireId.id()].MapT.Elem
			keyOp := dec.decIgnoreOpFor(keyId, inProgress)
			elemOp := dec.decIgnoreOpFor(elemId, inProgress)
			op = func(state *decoderState, value reflect.Value) {
				state.dec.ignoreMap(state, *keyOp, *elemOp)
			}

		case wire.SliceT != nil:
			elemId := wire.SliceT.Elem
			elemOp := dec.decIgnoreOpFor(elemId, inProgress)
			op = func(state *decoderState, value reflect.Value) {
				state.dec.ignoreSlice(state, *elemOp)
			}

		case wire.StructT != nil:
			// Generate a closure that calls out to the engine for the nested type.
			engine, err := dec.getIgnoreEnginePtr(wireId)
			if err != nil {
				error_(err)
			}
			op = func(state *decoderState, value reflect.Value) {
				// indirect through enginePtr to delay evaluation for recursive structs
				state.dec.ignoreStruct(engine)
			}

		case wire.GobEncoderT != nil, wire.BinaryMarshalerT != nil, wire.TextMarshalerT != nil:
			op = func(state *decoderState, value reflect.Value) {
				state.dec.ignoreGobDecoder(state)
			}
		}
	}
	if op == nil {
		errorf("bad data: ignore can't handle type %s", wireId.string())
	}
	return &op
}

// gobDecodeOpFor returns the op for a type that is known to implement
// GobDecoder.
func (dec *Decoder) gobDecodeOpFor(ut *userTypeInfo) *decOp {
	rcvrType := ut.user
	if ut.decIndir == -1 {
		rcvrType = reflect.PointerTo(rcvrType)
	} else if ut.decIndir > 0 {
		for i := int8(0); i < ut.decIndir; i++ {
			rcvrType = rcvrType.Elem()
		}
	}
	var op decOp
	op = func(state *decoderState, value reflect.Value) {
		// We now have the base type. We need its address if the receiver is a pointer.
		if value.Kind() != reflect.Pointer && rcvrType.Kind() == reflect.Pointer {
			value = value.Addr()
		}
		state.dec.decodeGobDecoder(ut, state, value)
	}
	return &op
}

// compatibleType asks: Are these two gob Types compatible?
// Answers the question for basic types, arrays, maps and slices, plus
// GobEncoder/Decoder pairs.
// Structs are considered ok; fields will be checked later.
func (dec *Decoder) compatibleType(fr reflect.Type, fw typeId, inProgress map[reflect.Type]typeId) bool {
	if rhs, ok := inProgress[fr]; ok {
		return rhs.id() == fw.id()
	}
	inProgress[fr] = fw
	ut := userType(fr)
	wire, ok := dec.wireType[fw.id()]
	// If wire was encoded with an encoding method, fr must have that method.
	// And if not, it must not.
	// At most one of the booleans in ut is set.
	// We could possibly relax this constraint in the future in order to
	// choose the decoding method using the data in the wireType.
	// The parentheses look odd but are correct.
	if (ut.externalDec == xGob) != (ok && wire.GobEncoderT != nil) ||
		(ut.externalDec == xBinary) != (ok && wire.BinaryMarshalerT != nil) ||
		(ut.externalDec == xText) != (ok && wire.TextMarshalerT != nil) {
		return false
	}
	if ut.externalDec != 0 { // This test trumps all others.
		return true
	}
	switch t := ut.base; t.Kind() {
	default:
		// chan, etc: cannot handle.
		return false
	case reflect.Bool:
		return fw.id() == tBool.id()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fw.id() == tInt.id()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return fw.id() == tUint.id()
	case reflect.Float32, reflect.Float64:
		return fw.id() == tFloat.id()
	case reflect.Complex64, reflect.Complex128:
		return fw.id() == tComplex.id()
	case reflect.String:
		return fw.id() == tString.id()
	case reflect.Interface:
		return fw.id() == tInterface.id()
	case reflect.Array:
		if !ok || wire.ArrayT == nil {
			return false
		}
		array := wire.ArrayT
		return t.Len() == array.Len && dec.compatibleType(t.Elem(), array.Elem, inProgress)
	case reflect.Map:
		if !ok || wire.MapT == nil {
			return false
		}
		MapType := wire.MapT
		return dec.compatibleType(t.Key(), MapType.Key, inProgress) && dec.compatibleType(t.Elem(), MapType.Elem, inProgress)
	case reflect.Slice:
		// Is it an array of bytes?
		if t.Elem().Kind() == reflect.Uint8 {
			return fw.id() == tBytes.id()
		}
		// Extract and compare element types.
		var sw *sliceType
		if tt := builtinIdToType(fw); tt != nil {
			sw, _ = tt.(*sliceType)
		} else if wire != nil {
			sw = wire.SliceT
		}
		elem := userType(t.Elem()).base
		return sw != nil && dec.compatibleType(elem, sw.Elem, inProgress)
	case reflect.Struct:
		return true
	}
}

// typeString returns a human-readable description of the type identified by remoteId.
func (dec *Decoder) typeString(remoteId typeId) string {
	typeLock.Lock()
	defer typeLock.Unlock()
	if t := idToType(remoteId); t != nil {
		// globally known type.
		return t.string()
	}
	return dec.wireType[remoteId.id()].string()
}

// getDecEnginePtr returns the engine for the specified type.
func (dec *Decoder) getDecEnginePtr(remoteId typeId, ut *userTypeInfo) (engine *decEngine, err error) {
	rt := ut.user
	decoderMap, ok := dec.decoderCache[rt]
	if !ok {
		decoderMap = make(map[typeId]*decEngine)
		dec.decoderCache[rt] = decoderMap
	}
	if engine, ok = decoderMap[remoteId]; !ok {
		// To handle recursive types, mark this engine as underway before compiling.
		engine = new(decEngine)
		decoderMap[remoteId] = engine
		err = engine.compileDec(dec, remoteId, ut)
		if err != nil {
			delete(decoderMap, remoteId)
		}
	}
	return
}

// emptyStruct is the type we compile into when ignoring a struct value.
type emptyStruct struct{}

var emptyStructType = reflect.TypeFor[emptyStruct]()

// getIgnoreEnginePtr returns the engine for the specified type when the value is to be discarded.
func (dec *Decoder) getIgnoreEnginePtr(wireId typeId) (engine *decEngine, err error) {
	var ok bool
	if engine, ok = dec.ignorerCache[wireId]; !ok {
		// To handle recursive types, mark this engine as underway before compiling.
		engine = new(decEngine)
		dec.ignorerCache[wireId] = engine
		wire := dec.wireType[wireId.id()]
		if wire != nil && wire.StructT != nil {
			err = engine.compileDec(dec, wireId, userType(emptyStructType))
		} else {
			engine.compileIgnoreSingle(dec, wireId)
		}
		if err != nil {
			delete(dec.ignorerCache, wireId)
		}
	}
	return
}

// decodeValue decodes the data stream representing a value and stores it in value.
func (dec *Decoder) decodeValue(wireId typeId, value reflect.Value) {
	defer catchError(&dec.err)
	// If the value is nil, it means we should just ignore this item.
	if !value.IsValid() {
		dec.decodeIgnoredValue(wireId)
		return
	}
	// Dereference down to the underlying type.
	ut := userType(value.Type())
	base := ut.base
	var engine *decEngine
	engine, dec.err = dec.getDecEnginePtr(wireId, ut)
	if dec.err != nil {
		return
	}
	value = decAlloc(value)
	if st := base; st.Kind() == reflect.Struct && ut.externalDec == 0 {
		wt := dec.wireType[wireId.id()]
		if engine.numInstr == 0 && st.NumField() > 0 &&
			wt != nil && len(wt.StructT.Field) > 0 {
			name := base.Name()
			errorf("type mismatch: no fields matched compiling decoder for %s", name)
		}
		dec.decodeStruct(engine, value)
	} else {
		dec.decodeSingle(engine, value)
	}
}

// decodeIgnoredValue decodes the data stream representing a value of the specified type and discards it.
func (dec *Decoder) decodeIgnoredValue(wireId typeId) {
	var engine *decEngine
	engine, dec.err = dec.getIgnoreEnginePtr(wireId)
	if dec.err != nil {
		return
	}
	wire := dec.wireType[wireId.id()]
	if wire != nil && wire.StructT != nil {
		dec.ignoreStruct(engine)
	} else {
		dec.ignoreSingle(engine)
	}
}

const (
	intBits     = 32 << (^uint(0) >> 63)
	uintptrBits = 32 << (^uintptr(0) >> 63)
)

func init() {
	var iop, uop decOp
	switch intBits {
	case 32:
		iop = decInt32
		uop = decUint32
	case 64:
		iop = decInt64
		uop = decUint64
	default:
		panic("gob: unknown size of int/uint")
	}
	decOpTable[reflect.Int] = iop
	decOpTable[reflect.Uint] = uop

	// Finally uintptr
	switch uintptrBits {
	case 32:
		uop = decUint32
	case 64:
		uop = decUint64
	default:
		panic("gob: unknown size of uintptr")
	}
	decOpTable[reflect.Uintptr] = uop
}

// Gob depends on being able to take the address
// of zeroed Values it creates, so use this wrapper instead
// of the standard reflect.Zero.
// Each call allocates once.
func allocValue(t reflect.Type) reflect.Value {
	return reflect.New(t).Elem()
}
