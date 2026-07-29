package reflectex

import (
	"iter"
	"reflect"
)

// A StructTag is the tag string in a struct field.
//
// By convention, tag strings are a concatenation of
// optionally space-separated key:"value" pairs.
// Each key is a non-empty string consisting of non-control
// characters other than space (U+0020 ' '), quote (U+0022 '"'),
// and colon (U+003A ':').  Each value is quoted using U+0022 '"'
// characters and Go string literal syntax.
//
// DOC(iga): Here I implement a more permissive tag value with new lines and tabulations.
type StructTag reflect.StructTag

// Lookup returns the value associated with key in the tag string.
// If the key is present in the tag the value (which may be empty)
// is returned. Otherwise the returned value will be the empty string.
// The ok return value reports whether the value was explicitly set in
// the tag string. If the tag does not have the conventional format,
// the value returned by Lookup is unspecified.
//
// DOC(iga): Here I implement a more permissive tag value with new lines and tabulations.
func (tag StructTag) Lookup(key string) (value string, ok bool) {
	// When modifying this code, also update the validateStructTag code
	// in cmd/vet/structtag.go.

	isWs := func(ch byte) bool {
		return ch == ' ' || ch == '\n' || ch == '\t'
	}

	for tag != "" {
		// Skip leading space.
		i := 0

		for i < len(tag) && isWs(tag[i]) {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i])
		tag = tag[i+1:]

		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		qvalue := string(tag[:i+1])
		tag = tag[i+1:]

		if key == name {
			// strvcon.Unquote replaced with that to support multiline strings.
			value := qvalue
			if len(qvalue) > 1 && qvalue[0] == '"' && qvalue[len(qvalue)-1] == '"' {
				value = qvalue[1 : len(qvalue)-1]
			}

			return value, true
		}
	}
	return "", false
}

func (tag StructTag) Enum() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		isWs := func(ch byte) bool {
			return ch == ' ' || ch == '\n' || ch == '\t'
		}

		for tag != "" {
			// Skip leading space.
			i := 0

			for i < len(tag) && isWs(tag[i]) {
				i++
			}
			tag = tag[i:]
			if tag == "" {
				break
			}

			// Scan to colon. A space, a quote or a control character is a syntax error.
			// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
			// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
			// as it is simpler to inspect the tag's bytes than the tag's runes.
			i = 0
			for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
				i++
			}
			if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
				break
			}
			name := string(tag[:i])
			tag = tag[i+1:]

			// Scan quoted string to find value.
			i = 1
			for i < len(tag) && tag[i] != '"' {
				if tag[i] == '\\' {
					i++
				}
				i++
			}
			if i >= len(tag) {
				break
			}
			qvalue := string(tag[:i+1])
			tag = tag[i+1:]

			// strvcon.Unquote replaced with that to support multiline strings.
			value := qvalue
			if len(qvalue) > 1 && qvalue[0] == '"' && qvalue[len(qvalue)-1] == '"' {
				value = qvalue[1 : len(qvalue)-1]
			}

			if !yield(name, value) {
				return
			}
		}
	}
}
