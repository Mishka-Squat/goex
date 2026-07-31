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

// EmbeddedAddress is embedded (anonymous) in embeddedPersonRow to exercise
// Go struct embedding, as opposed to nestedAddress/nestedPersonRow above
// which use an explicit named field. It must be exported so its field name
// ("EmbeddedAddress") matches the PascalCase strcase.ToCamel conversion of
// the "embedded_address" header segment.
type EmbeddedAddress struct {
	Street string
	City   string
}

type embeddedPersonRow struct {
	Name string
	EmbeddedAddress
}

func TestHeaderToMap(t *testing.T) {
	h := headerToMap([]string{"name", "age"})

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
	row := make([]string, len(header))
	err := encoder.Encode(item, row)
	require.NoError(t, err)

	assert.Equal(t, []string{"Igor", "42", "3.5", "true"}, row)

	decoder := MakeDecoder[simpleRow](header)
	decoded, err := decoder.Decode(row)
	require.NoError(t, err)

	assert.Equal(t, item, decoded)
}

func TestEncodeDecodeCustomType(t *testing.T) {
	header := []string{"name", "color"}

	item := coloredRow{Name: "sky", Color: testColor{R: 10, G: 20, B: 30}}

	encoder := MakeEncoder[coloredRow](header)
	row := make([]string, len(header))
	err := encoder.Encode(item, row)
	require.NoError(t, err)

	assert.Equal(t, []string{"sky", "10/20/30"}, row)

	decoder := MakeDecoder[coloredRow](header)
	decoded, err := decoder.Decode(row)
	require.NoError(t, err)

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
	h := headerToMap([]string{"name", "address.street", "address.city"})

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
	row := make([]string, len(header))
	err := encoder.Encode(item, row)
	require.NoError(t, err)

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
		var err error
		decoded, err = decoder.Decode([]string{"Igor", "Baker St", "Metropolis"})
		require.NoError(t, err)
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

// terrainKind mimics a real game enum type (backed by a primitive kind but
// implementing CsvParser/CsvFormatter), which exercises the same code path
// as production types like colonization.TerrainType.
type terrainKind uint8

const (
	terrainTundra terrainKind = iota
	terrainScrubForest
)

func (t terrainKind) String() string {
	if t == terrainScrubForest {
		return "ScrubForest"
	}
	return "Tundra"
}

func (t terrainKind) ParseToAny(s string) any {
	if s == "ScrubForest" {
		return terrainScrubForest
	}
	return terrainTundra
}

type enumRow struct {
	Name    string
	Terrain terrainKind
}

type resourceKind uint8

const (
	resourceFood resourceKind = iota
	resourceOre
)

func (t resourceKind) String() string {
	if t == resourceFood {
		return "Food"
	}
	return "Ore"
}

func (t resourceKind) ParseToAny(s string) any {
	if s == "Food" {
		return resourceFood
	}
	return resourceOre
}

type mapTileRow struct {
	Terrain     terrainKind
	Name        string
	Yield       map[resourceKind]uint8
	IsForest    bool
	IsMountains bool
}

var mapTileCsvHeader = []string{
	"terrain", "name", "yield.Food", "yield.Ore", "is_forest", "is_mountains",
}

const mapTileCsv = "id,terrain,name,yield.Food,yield.Ore,is_forest,is_mountains\n" +
	"0,Tundra,Tundra,2,2,false,false\n" +
	"9,ScrubForest,Scrub Forest,1,1,true,false\n" +
	"19,Tundra,Mountains,0,4,false,true\n"

// TestReadCsvTable_MapTile exercises decoding a dotted header segment
// ("yield.Food"/"yield.Ore") into a map field keyed by an enum that
// implements CsvParser, on a realistic, game-shaped struct.
func TestReadCsvTable_MapTile(t *testing.T) {
	rows, err := ReadCsvTable[mapTileRow](strings.NewReader(mapTileCsv))
	require.NoError(t, err)

	want := []CsvRow[mapTileRow]{
		{Id: "0", T: mapTileRow{
			Terrain: terrainTundra,
			Name:    "Tundra",
			Yield:   map[resourceKind]uint8{resourceFood: 2, resourceOre: 2},
		}},
		{Id: "9", T: mapTileRow{
			Terrain:  terrainScrubForest,
			Name:     "Scrub Forest",
			Yield:    map[resourceKind]uint8{resourceFood: 1, resourceOre: 1},
			IsForest: true,
		}},
		{Id: "19", T: mapTileRow{
			Terrain:     terrainTundra,
			Name:        "Mountains",
			Yield:       map[resourceKind]uint8{resourceFood: 0, resourceOre: 4},
			IsMountains: true,
		}},
	}
	assert.Equal(t, want, rows)
}

// TestWriteCsvTable_MapTile exercises encoding a map field keyed by an enum
// that implements CsvFormatter into dotted header columns
// ("yield.Food"/"yield.Ore"), alongside a plain CsvFormatter leaf (Terrain).
func TestWriteCsvTable_MapTile(t *testing.T) {
	items := []CsvRow[mapTileRow]{
		{Id: "9", T: mapTileRow{
			Terrain: terrainScrubForest,
			Name:    "Scrub Forest",
			Yield: map[resourceKind]uint8{
				resourceFood: 3,
				resourceOre:  1,
			},
			IsForest: true,
		}},
	}

	var buf strings.Builder
	err := WriteCsvTable(&buf, mapTileCsvHeader, items)
	require.NoError(t, err)

	want := "id,terrain,name,yield.Food,yield.Ore,is_forest,is_mountains\n" +
		"9,ScrubForest,Scrub Forest,3,1,true,false\n"
	assert.Equal(t, want, buf.String())
}

// leafStats and midStats give the CSV engine a third level of header
// nesting ("mid.leaf.power"), which no production type currently has.
type leafStats struct {
	Power uint8
}

type midStats struct {
	Armor uint8
	Leaf  leafStats
}

type miniTile struct {
	Name string
	Mid  midStats
}

func TestReadWriteCsvTable_NestedStruct(t *testing.T) {
	header := []string{"name", "mid.armor", "mid.leaf.power"}
	items := []CsvRow[miniTile]{
		{Id: "1", T: miniTile{Name: "Frigate", Mid: midStats{Armor: 5, Leaf: leafStats{Power: 9}}}},
	}

	var buf strings.Builder
	require.NoError(t, WriteCsvTable(&buf, header, items))
	assert.Equal(t, "id,name,mid.armor,mid.leaf.power\n1,Frigate,5,9\n", buf.String())

	rows, err := ReadCsvTable[miniTile](strings.NewReader(buf.String()))
	require.NoError(t, err)
	assert.Equal(t, items, rows)
}
