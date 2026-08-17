// This file contains tests of the GobEncoderEx/GobDecoderEx support.

package gobex

import (
	"bytes"
	"reflect"
	"testing"
)

// ExItem is a plain value nested inside an ExGobber via the Encoder/Decoder
// passed to GobEncodeEx/GobDecodeEx, mirroring how ecs.Grid[T] stores its
// cells.
type ExItem struct {
	A int
	B string
}

// ExGobber mirrors ecs.Grid[T]'s GobEncoderEx/GobDecoderEx usage: encoding
// writes a count followed by each item via the passed-in *Encoder, decoding
// reads them back the same way via the passed-in *Decoder. The value/pointer
// receiver split matches Grid[T] exactly (value receiver for encode, pointer
// receiver for decode), since Grid[T] is embedded as a value field.
type ExGobber struct {
	Items []ExItem
}

func (g ExGobber) GobEncodeEx(enc *Encoder) error {
	if err := enc.Encode(len(g.Items)); err != nil {
		return err
	}
	for _, item := range g.Items {
		if err := enc.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

func (g *ExGobber) GobDecodeEx(dec *Decoder) error {
	var n int
	if err := dec.Decode(&n); err != nil {
		return err
	}
	g.Items = make([]ExItem, n)
	for i := range g.Items {
		if err := dec.Decode(&g.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

// GobTestEx has ordinary fields both before and after the GobEncoderEx
// field, so that a GobEncodeEx implementation which writes to the wrong
// destination (e.g. straight to the underlying stream instead of into the
// field's own buffered blob) corrupts the surrounding struct's field order
// and is caught here.
type GobTestEx struct {
	X int
	G ExGobber
	Y int
}

func TestGobEncoderExField(t *testing.T) {
	b := new(bytes.Buffer)
	enc := NewEncoder(b)
	in := GobTestEx{
		X: 17,
		G: ExGobber{Items: []ExItem{{A: 1, B: "one"}, {A: 2, B: "two"}}},
		Y: 42,
	}
	if err := enc.Encode(in); err != nil {
		t.Fatal("encode error:", err)
	}

	dec := NewDecoder(b)
	out := new(GobTestEx)
	if err := dec.Decode(out); err != nil {
		t.Fatal("decode error:", err)
	}

	if out.X != in.X {
		t.Errorf("X: expected %d got %d", in.X, out.X)
	}
	if out.Y != in.Y {
		t.Errorf("Y: expected %d got %d", in.Y, out.Y)
	}
	if !reflect.DeepEqual(in.G, out.G) {
		t.Errorf("G: expected %+v got %+v", in.G, out.G)
	}
}

// TestGobEncoderExFieldEmpty checks the zero-items case, which still needs
// the length prefix to round-trip correctly.
func TestGobEncoderExFieldEmpty(t *testing.T) {
	b := new(bytes.Buffer)
	enc := NewEncoder(b)
	in := GobTestEx{X: 1, G: ExGobber{Items: nil}, Y: 2}
	if err := enc.Encode(in); err != nil {
		t.Fatal("encode error:", err)
	}

	dec := NewDecoder(b)
	out := new(GobTestEx)
	if err := dec.Decode(out); err != nil {
		t.Fatal("decode error:", err)
	}
	if out.X != in.X || out.Y != in.Y {
		t.Errorf("expected X=%d Y=%d got X=%d Y=%d", in.X, in.Y, out.X, out.Y)
	}
	if len(out.G.Items) != 0 {
		t.Errorf("expected no items, got %+v", out.G.Items)
	}
}

// TestGobEncoderExFieldRepeated encodes and decodes two independent
// GobTestEx values on a shared Encoder/Decoder pair, reusing the same
// pattern as TestGobEncoderField. Since ExItem is also sent as part of the
// first message, a GobEncodeEx implementation whose isolated sub-message
// shares the outer Encoder's "already sent" type registry would wrongly
// omit ExItem's type descriptor from the second message, so this also
// covers the type-registry independence of the per-field sub-encoder.
func TestGobEncoderExFieldRepeated(t *testing.T) {
	b := new(bytes.Buffer)
	enc := NewEncoder(b)
	dec := NewDecoder(b)

	first := GobTestEx{X: 1, G: ExGobber{Items: []ExItem{{A: 1, B: "a"}}}, Y: 2}
	if err := enc.Encode(first); err != nil {
		t.Fatal("encode error:", err)
	}
	firstOut := new(GobTestEx)
	if err := dec.Decode(firstOut); err != nil {
		t.Fatal("decode error:", err)
	}
	if !reflect.DeepEqual(first, *firstOut) {
		t.Errorf("first: expected %+v got %+v", first, *firstOut)
	}

	second := GobTestEx{X: 3, G: ExGobber{Items: []ExItem{{A: 2, B: "b"}, {A: 3, B: "c"}}}, Y: 4}
	if err := enc.Encode(second); err != nil {
		t.Fatal("encode error:", err)
	}
	secondOut := new(GobTestEx)
	if err := dec.Decode(secondOut); err != nil {
		t.Fatal("decode error:", err)
	}
	if !reflect.DeepEqual(second, *secondOut) {
		t.Errorf("second: expected %+v got %+v", second, *secondOut)
	}
}

// TestGobEncoderExFieldSlice encodes a slice of structs each containing a
// GobEncoderEx field, exercising several isolated sub-messages within a
// single top-level Encode call.
func TestGobEncoderExFieldSlice(t *testing.T) {
	b := new(bytes.Buffer)
	enc := NewEncoder(b)
	in := []GobTestEx{
		{X: 1, G: ExGobber{Items: []ExItem{{A: 1, B: "a"}}}, Y: 2},
		{X: 3, G: ExGobber{Items: []ExItem{{A: 2, B: "b"}, {A: 3, B: "c"}}}, Y: 4},
		{X: 5, G: ExGobber{Items: nil}, Y: 6},
	}
	if err := enc.Encode(in); err != nil {
		t.Fatal("encode error:", err)
	}

	dec := NewDecoder(b)
	var out []GobTestEx
	if err := dec.Decode(&out); err != nil {
		t.Fatal("decode error:", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("expected %+v got %+v", in, out)
	}
}

// DynOnlyItem's type is reachable only through the any/DynAny() dynamic
// dispatch inside DynGobber.GobEncodeEx/GobDecodeEx below, never as a
// declared field of any statically-reflected type (unlike ExItem above,
// which IS a declared field of ExGobber and therefore gets its type
// descriptor sent as part of the outer struct's ordinary preamble scan).
// This mirrors ecs.Grid[T]'s real cell type: it's produced dynamically via
// DtoAny(), so its type descriptor can only be discovered and sent for the
// first time from *inside* an isolated GobEncoderEx sub-message.
type DynOnlyItem struct {
	A int
	B string
}

type dynAny interface {
	DynAny() any
}

func (d DynOnlyItem) DynAny() any { return d }

type DynGobber struct {
	items []DynOnlyItem
}

func (g DynGobber) GobEncodeEx(enc *Encoder) error {
	if err := enc.Encode(len(g.items)); err != nil {
		return err
	}
	for _, it := range g.items {
		if err := enc.Encode(any(it).(dynAny).DynAny()); err != nil {
			return err
		}
	}
	return nil
}

func (g *DynGobber) GobDecodeEx(dec *Decoder) error {
	var n int
	if err := dec.Decode(&n); err != nil {
		return err
	}
	g.items = make([]DynOnlyItem, n)
	if n == 0 {
		return nil
	}
	dv := any(DynOnlyItem{}).(dynAny).DynAny()
	data := reflect.New(reflect.ValueOf(dv).Type())
	for i := range g.items {
		if err := dec.DecodeValue(data); err != nil {
			return err
		}
		g.items[i] = data.Elem().Interface().(DynOnlyItem)
	}
	return nil
}

type DynWrap struct {
	X int
	G DynGobber
	Y int
}

// TestGobEncoderExDynamicOnlyType guards against a real bug found in
// decOpFor's reflect.Struct case: the compiled op closure for a nested
// struct field called `dec.decodeStruct(engine, value)` using the *Decoder
// that happened to compile the engine, instead of `state.dec` (the decoder
// actually performing the current decode). Decode engines are cached in
// Decoder.decoderCache, which GobDecoderEx's isolated per-field sub-decoder
// deliberately shares with the outer decoder (see decodeGobDecoder's xGobEx
// case) so type descriptors don't need re-sending. That sharing means an
// engine compiled once by the outer decoder gets reused by every isolated
// sub-decoder — and the stale closure silently decoded from the *outer*
// decoder's buffer instead of the sub-decoder's, producing "gob: bad data:
// field numbers out of bounds" (or worse, silently wrong data) as soon as a
// type's *wireType* descriptor was first decoded from within an isolated
// blob rather than the main stream. Only reachable when a struct nested
// inside a wireType payload (which is any user struct type descriptor) gets
// decoded for the first time by a sub-decoder, which is exactly what
// happens the first time a dynamically-dispatched type like DynOnlyItem (or
// ecs.Grid[T]'s real DTO cell type) is encoded.
func TestGobEncoderExDynamicOnlyType(t *testing.T) {
	in := DynWrap{X: 1, G: DynGobber{items: []DynOnlyItem{{A: 1, B: "a"}, {A: 2, B: "b"}}}, Y: 2}
	b := new(bytes.Buffer)
	enc := NewEncoder(b)
	if err := enc.Encode(in); err != nil {
		t.Fatal("encode error:", err)
	}
	dec := NewDecoder(b)
	out := new(DynWrap)
	if err := dec.Decode(out); err != nil {
		t.Fatal("decode error:", err)
	}
	if out.X != in.X || out.Y != in.Y {
		t.Errorf("expected X=%d Y=%d got X=%d Y=%d", in.X, in.Y, out.X, out.Y)
	}
	if !reflect.DeepEqual(in.G.items, out.G.items) {
		t.Errorf("G.items: expected %+v got %+v", in.G.items, out.G.items)
	}
}
