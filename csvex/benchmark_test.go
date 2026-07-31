package csvex

import "testing"

func BenchmarkEncode(b *testing.B) {
	header := []string{"name", "age", "score", "active"}
	item := simpleRow{Name: "Igor", Age: 42, Score: 3.5, Active: true}
	encoder := MakeEncoder[simpleRow](header)

	for b.Loop() {
		_, _ = encoder.Encode(item)
	}
}

func BenchmarkSillyEncode(b *testing.B) {
	header := []string{"name", "age", "score", "active"}
	item := simpleRow{Name: "Igor", Age: 42, Score: 3.5, Active: true}

	for b.Loop() {
		encoder := MakeEncoder[simpleRow](header)
		_, _ = encoder.Encode(item)
	}
}

func BenchmarkDecode(b *testing.B) {
	header := []string{"name", "age", "score", "active"}
	row := []string{"Igor", "42", "3.5", "true"}
	decoder := MakeDecoder[simpleRow](header)

	for b.Loop() {
		_, _ = decoder.Decode(row)
	}
}

func BenchmarkSillyDecode(b *testing.B) {
	header := []string{"name", "age", "score", "active"}
	row := []string{"Igor", "42", "3.5", "true"}

	for b.Loop() {
		decoder := MakeDecoder[simpleRow](header)
		_, _ = decoder.Decode(row)
	}
}

func BenchmarkEncodeCustomType(b *testing.B) {
	header := []string{"name", "color"}
	item := coloredRow{Name: "sky", Color: testColor{R: 10, G: 20, B: 30}}
	encoder := MakeEncoder[coloredRow](header)

	for b.Loop() {
		_, _ = encoder.Encode(item)
	}
}

func BenchmarkSillyEncodeCustomType(b *testing.B) {
	header := []string{"name", "color"}
	item := coloredRow{Name: "sky", Color: testColor{R: 10, G: 20, B: 30}}

	for b.Loop() {
		encoder := MakeEncoder[coloredRow](header)
		_, _ = encoder.Encode(item)
	}
}

func BenchmarkDecodeCustomType(b *testing.B) {
	header := []string{"name", "color"}
	row := []string{"sky", "10/20/30"}
	decoder := MakeDecoder[coloredRow](header)

	for b.Loop() {
		_, _ = decoder.Decode(row)
	}
}

func BenchmarkSillyDecodeCustomType(b *testing.B) {
	header := []string{"name", "color"}
	row := []string{"sky", "10/20/30"}

	for b.Loop() {
		decoder := MakeDecoder[coloredRow](header)
		_, _ = decoder.Decode(row)
	}
}

func BenchmarkEncodeEmbedded(b *testing.B) {
	header := []string{"name", "embedded_address.street", "embedded_address.city"}
	item := embeddedPersonRow{Name: "Igor", EmbeddedAddress: EmbeddedAddress{Street: "Baker St", City: "Metropolis"}}
	encoder := MakeEncoder[embeddedPersonRow](header)

	for b.Loop() {
		_, _ = encoder.Encode(item)
	}
}

func BenchmarkSillyEncodeEmbedded(b *testing.B) {
	header := []string{"name", "embedded_address.street", "embedded_address.city"}
	item := embeddedPersonRow{Name: "Igor", EmbeddedAddress: EmbeddedAddress{Street: "Baker St", City: "Metropolis"}}

	for b.Loop() {
		encoder := MakeEncoder[embeddedPersonRow](header)
		_, _ = encoder.Encode(item)
	}
}

func BenchmarkDecodeEmbedded(b *testing.B) {
	header := []string{"name", "embedded_address.street", "embedded_address.city"}
	row := []string{"Igor", "Baker St", "Metropolis"}
	decoder := MakeDecoder[embeddedPersonRow](header)

	for b.Loop() {
		_, _ = decoder.Decode(row)
	}
}

func BenchmarkSillyDecodeEmbedded(b *testing.B) {
	header := []string{"name", "embedded_address.street", "embedded_address.city"}
	row := []string{"Igor", "Baker St", "Metropolis"}

	for b.Loop() {
		decoder := MakeDecoder[embeddedPersonRow](header)
		_, _ = decoder.Decode(row)
	}
}

func BenchmarkEncodeEnum(b *testing.B) {
	header := []string{"name", "terrain"}
	item := enumRow{Name: "Scrub Forest", Terrain: terrainScrubForest}
	encoder := MakeEncoder[enumRow](header)

	for b.Loop() {
		_, _ = encoder.Encode(item)
	}
}

func BenchmarkSillyEncodeEnum(b *testing.B) {
	header := []string{"name", "terrain"}
	item := enumRow{Name: "Scrub Forest", Terrain: terrainScrubForest}

	for b.Loop() {
		encoder := MakeEncoder[enumRow](header)
		_, _ = encoder.Encode(item)
	}
}

func BenchmarkDecodeEnum(b *testing.B) {
	header := []string{"name", "terrain"}
	row := []string{"Scrub Forest", "ScrubForest"}
	decoder := MakeDecoder[enumRow](header)

	for b.Loop() {
		_, _ = decoder.Decode(row)
	}
}

func BenchmarkSillyDecodeEnum(b *testing.B) {
	header := []string{"name", "terrain"}
	row := []string{"Scrub Forest", "ScrubForest"}

	for b.Loop() {
		decoder := MakeDecoder[enumRow](header)
		_, _ = decoder.Decode(row)
	}
}
