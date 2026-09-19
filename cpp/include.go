// Package cpp implements a subset of the C preprocessor that is useful for
// text sources such as GLSL shaders: #include expansion.
package cpp

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"regexp"
)

// IncludeDepthLimit caps #include nesting, so a pathological chain of
// includes fails with an error instead of exhausting the stack.
const IncludeDepthLimit = 32

// MaxLineSize is the maximum length of a single source line the include
// scanner accepts. Generated sources (e.g. long #define blocks on one line)
// can exceed bufio's 64KiB default.
const MaxLineSize = 1 << 20

// includeRe matches an #include directive:
//
//	#include "other.glsl"   — resolved relative to the including file
//	#include <other.glsl>   — resolved from the FS root
var includeRe = regexp.MustCompile(`^[ \t]*#[ \t]*include[ \t]+(?:"([^"]*)"|<([^>]*)>)[ \t]*(?://.*)?\r?$`)

// ExpandIncludes reads the source name from fsys and replaces every
// #include directive with the content of the included file, recursively. It
// returns the expanded source and the paths of the included files in
// expansion order.
//
// A quoted include path is relative to the directory of the file containing
// the directive, an angle-bracketed one is relative to the FS root. Each
// file is included at most once per expansion (like an implicit
// #pragma once), so shared headers need no include guards and include
// cycles terminate instead of looping.
func ExpandIncludes(fsys fs.FS, name string) (src []byte, includes []string, err error) {
	st := includeState{
		fsys:     fsys,
		included: map[string]struct{}{},
	}

	if err = st.expand(path.Clean(name), 0); err != nil {
		return nil, nil, err
	}

	return st.out.Bytes(), st.includes, nil
}

// includeState is the shared state of a single ExpandIncludes run.
type includeState struct {
	out      bytes.Buffer
	fsys     fs.FS
	included map[string]struct{}
	includes []string
}

// NewLineScanner returns a line scanner over src that accepts lines up to
// MaxLineSize long.
func NewLineScanner(src []byte) *bufio.Scanner {
	scanner := bufio.NewScanner(bytes.NewReader(src))
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), MaxLineSize)
	return scanner
}

func (st *includeState) expand(name string, depth int) error {
	if depth > IncludeDepthLimit {
		return fmt.Errorf("%s: #include nested deeper than %d levels", name, IncludeDepthLimit)
	}

	src, err := fs.ReadFile(st.fsys, name)
	if err != nil {
		return err
	}

	st.included[name] = struct{}{}
	if depth > 0 {
		st.includes = append(st.includes, name)
	}

	scanner := NewLineScanner(src)

	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()

		target, ok := ParseIncludeDirective(text, name)
		if !ok {
			st.out.WriteString(text)
			st.out.WriteByte('\n')
			continue
		}

		if _, done := st.included[target]; done {
			continue
		}

		if err := st.expand(target, depth+1); err != nil {
			return fmt.Errorf("%s:%d: include %q: %w", name, line, target, err)
		}
	}

	return scanner.Err()
}

// ParseIncludeDirective reports whether line is an #include directive in the
// file name and, if so, the included path resolved within the FS.
func ParseIncludeDirective(line string, name string) (target string, ok bool) {
	m := includeRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}

	if m[1] != "" {
		return path.Join(path.Dir(name), m[1]), true
	}

	if m[2] != "" {
		return path.Clean(m[2]), true
	}

	return "", false
}
