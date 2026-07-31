package csvex

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Mishka-Squat/orderedmap/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type simpleRow struct {
	Name   string
	Age    int
	Score  float64
	Active bool
}

// testColor implements both CsvParser (decode) and CsvFormatter (encode),
// which exercises the "leaf" branches of MakeDecoderAny/MakeEncoderAny.
type testColor struct {
	R, G, B int
}

func (c testColor) String() string {
	return fmt.Sprintf("%d/%d/%d", c.R, c.G, c.B)
}

func (c testColor) ParseToAny(s string) any {
	var r, g, b int
	fmt.Sscanf(s, "%d/%d/%d", &r, &g, &b)
	return testColor{R: r, G: g, B: b}
}

type coloredRow struct {
	Name  string
	Color testColor
}

type nestedAddress struct {
	Street string
	City   string
}

type nestedPersonRow struct {
	Name    string
	Address nestedAddress
}

func TestHeaderToMap(t *testing.T) {
	h := HeaderToMap([]string{"name", "age"})

	got := []string{}
	for k := range h.All() {
		got = append(got, k)
	}

	assert.Equal(t, []string{"name", "age"}, got)
}

func TestEncodeDecodeScalars(t *testing.T) {
	header := []string{"name", "age", "score", "active"}

	item := simpleRow{Name: "Igor", Age: 42, Score: 3.5, Active: true}

	encoder := MakeEncoder[simpleRow](header)
	row := encoder.Encode(item)

	assert.Equal(t, []string{"Igor", "42", "3.5", "true"}, row)

	decoder := MakeDecoder[simpleRow](header)
	decoded := decoder.Decode(row)

	assert.Equal(t, item, decoded)
}

func TestEncodeDecodeCustomType(t *testing.T) {
	header := []string{"name", "color"}

	item := coloredRow{Name: "sky", Color: testColor{R: 10, G: 20, B: 30}}

	encoder := MakeEncoder[coloredRow](header)
	row := encoder.Encode(item)

	assert.Equal(t, []string{"sky", "10/20/30"}, row)

	decoder := MakeDecoder[coloredRow](header)
	decoded := decoder.Decode(row)

	assert.Equal(t, item, decoded)
}

// The tests below assert correct nested-struct ("address.city" style) header
// support. They currently FAIL against csv_reader.go because of two bugs:
//   - HeaderMap.set overwrites the nested HeaderMap for every dotted header
//     segment instead of merging into the one already stored under that key,
//     so only the last nested field of a given struct survives.
//   - Encode/Decode index struct fields by position in the flattened Ops
//     slice (v.Field(i)), which only lines up with the target struct's fields
//     when every header maps 1:1 to a top-level field. A single nested field
//     breaks that assumption.

func TestHeaderToMapNestedFields(t *testing.T) {
	h := HeaderToMap([]string{"name", "address.street", "address.city"})

	addr, ok := h.Get("address")
	require.True(t, ok, "HeaderToMap() missing %q key", "address")

	addrMap, ok := addr.(orderedmap.Of[string, any])
	require.True(t, ok, "HeaderToMap()[%q] = %T, want orderedmap.Of[string, any]", "address", addr)

	assert.True(t, addrMap.Has("street"), "nested map missing %q", "street")
	assert.True(t, addrMap.Has("city"), "nested map missing %q", "city")
}

func TestEncodeNestedStruct(t *testing.T) {
	header := []string{"name", "address.street", "address.city"}
	item := nestedPersonRow{Name: "Igor", Address: nestedAddress{Street: "Baker St", City: "Metropolis"}}

	encoder := MakeEncoder[nestedPersonRow](header)
	row := encoder.Encode(item)

	assert.Equal(t, []string{"Igor", "Baker St", "Metropolis"}, row)
}

func TestDecodeNestedStruct(t *testing.T) {
	header := []string{"name", "address.street", "address.city"}
	item := nestedPersonRow{Name: "Igor", Address: nestedAddress{Street: "Baker St", City: "Metropolis"}}

	decoder := MakeDecoder[nestedPersonRow](header)

	var decoded nestedPersonRow
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Decode() panicked: %v", r)
			}
		}()
		decoded = decoder.Decode([]string{"Igor", "Baker St", "Metropolis"})
	}()

	assert.Equal(t, item, decoded)
}

func TestWriteCsvTable(t *testing.T) {
	header := []string{"name", "age", "score", "active"}
	items := []CsvRow[simpleRow]{
		{Id: "1", T: simpleRow{Name: "Igor", Age: 42, Score: 3.5, Active: true}},
		{Id: "2", T: simpleRow{Name: "Anna", Age: 30, Score: 1.25, Active: false}},
	}

	var buf strings.Builder
	require.NoError(t, WriteCsvTable(&buf, header, items))

	want := "id,name,age,score,active\n" +
		"1,Igor,42,3.5,true\n" +
		"2,Anna,30,1.25,false\n"

	assert.Equal(t, want, buf.String())
}

func TestWriteThenReadCsvTable(t *testing.T) {
	header := []string{"name", "age", "score", "active"}
	items := []CsvRow[simpleRow]{
		{Id: "1", T: simpleRow{Name: "Igor", Age: 42, Score: 3.5, Active: true}},
		{Id: "2", T: simpleRow{Name: "Anna", Age: 30, Score: 1.25, Active: false}},
	}

	var buf strings.Builder
	require.NoError(t, WriteCsvTable(&buf, header, items))

	got, err := ReadCsvTable[simpleRow](strings.NewReader(buf.String()))
	require.NoError(t, err)

	assert.Equal(t, items, got)
}

func BenchmarkEncode(b *testing.B) {
	header := []string{"name", "age", "score", "active"}
	item := simpleRow{Name: "Igor", Age: 42, Score: 3.5, Active: true}
	encoder := MakeEncoder[simpleRow](header)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = encoder.Encode(item)
	}
}

func BenchmarkDecode(b *testing.B) {
	header := []string{"name", "age", "score", "active"}
	row := []string{"Igor", "42", "3.5", "true"}
	decoder := MakeDecoder[simpleRow](header)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(row)
	}
}

func BenchmarkEncodeCustomType(b *testing.B) {
	header := []string{"name", "color"}
	item := coloredRow{Name: "sky", Color: testColor{R: 10, G: 20, B: 30}}
	encoder := MakeEncoder[coloredRow](header)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = encoder.Encode(item)
	}
}

func BenchmarkDecodeCustomType(b *testing.B) {
	header := []string{"name", "color"}
	row := []string{"sky", "10/20/30"}
	decoder := MakeDecoder[coloredRow](header)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(row)
	}
}
