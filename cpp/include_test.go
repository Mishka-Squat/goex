package cpp

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestExpandIncludesQuotedPathIsRelativeToIncludingFile(t *testing.T) {
	fsys := fstest.MapFS{
		"shaders/map.fs.glsl":    &fstest.MapFile{Data: []byte("#version 330\n#include \"lib/color.glsl\"\nvoid main() {}\n")},
		"shaders/lib/color.glsl": &fstest.MapFile{Data: []byte("vec3 gray() { return vec3(0.5); }\n")},
	}

	src, _, err := ExpandIncludes(fsys, "shaders/map.fs.glsl")
	if err != nil {
		t.Fatalf("ExpandIncludes: %v", err)
	}

	want := "#version 330\nvec3 gray() { return vec3(0.5); }\nvoid main() {}\n"
	if got := string(src); got != want {
		t.Errorf("expanded source:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestExpandIncludesAngleBracketPathIsRelativeToRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"shaders/lib/a.glsl": &fstest.MapFile{Data: []byte("#include <shaders/lib/b.glsl>\nA\n")},
		"shaders/lib/b.glsl": &fstest.MapFile{Data: []byte("B\n")},
	}

	src, _, err := ExpandIncludes(fsys, "shaders/lib/a.glsl")
	if err != nil {
		t.Fatalf("ExpandIncludes: %v", err)
	}

	if got, want := string(src), "B\nA\n"; got != want {
		t.Errorf("expanded source: got %q, want %q", got, want)
	}
}

func TestExpandIncludesIsRecursive(t *testing.T) {
	fsys := fstest.MapFS{
		"a.glsl": &fstest.MapFile{Data: []byte("#include \"b.glsl\"\nA\n")},
		"b.glsl": &fstest.MapFile{Data: []byte("#include \"c.glsl\"\nB\n")},
		"c.glsl": &fstest.MapFile{Data: []byte("C\n")},
	}

	src, _, err := ExpandIncludes(fsys, "a.glsl")
	if err != nil {
		t.Fatalf("ExpandIncludes: %v", err)
	}

	if got, want := string(src), "C\nB\nA\n"; got != want {
		t.Errorf("expanded source: got %q, want %q", got, want)
	}
}

func TestExpandIncludesIncludesEachFileOnce(t *testing.T) {
	fsys := fstest.MapFS{
		"a.glsl": &fstest.MapFile{Data: []byte("#include \"c.glsl\"\n#include \"b.glsl\"\nA\n")},
		"b.glsl": &fstest.MapFile{Data: []byte("#include \"c.glsl\"\nB\n")},
		"c.glsl": &fstest.MapFile{Data: []byte("C\n")},
	}

	src, _, err := ExpandIncludes(fsys, "a.glsl")
	if err != nil {
		t.Fatalf("ExpandIncludes: %v", err)
	}

	if got, want := strings.Count(string(src), "C"), 1; got != want {
		t.Errorf("c.glsl expanded %d times, want %d: %q", got, want, src)
	}
}

func TestExpandIncludesTerminatesOnCycle(t *testing.T) {
	fsys := fstest.MapFS{
		"a.glsl": &fstest.MapFile{Data: []byte("#include \"b.glsl\"\nA\n")},
		"b.glsl": &fstest.MapFile{Data: []byte("#include \"a.glsl\"\nB\n")},
	}

	src, _, err := ExpandIncludes(fsys, "a.glsl")
	if err != nil {
		t.Fatalf("ExpandIncludes: %v", err)
	}

	if got, want := string(src), "B\nA\n"; got != want {
		t.Errorf("expanded source: got %q, want %q", got, want)
	}
}

func TestExpandIncludesReportsMissingFile(t *testing.T) {
	fsys := fstest.MapFS{
		"a.glsl": &fstest.MapFile{Data: []byte("A\n#include \"missing.glsl\"\n")},
	}

	_, _, err := ExpandIncludes(fsys, "a.glsl")
	if err == nil {
		t.Fatal("ExpandIncludes: expected an error for a missing include")
	}

	for _, want := range []string{"a.glsl:2", "missing.glsl"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestParseIncludeDirective(t *testing.T) {
	tests := []struct {
		line   string
		target string
		ok     bool
	}{
		{`#include "b.glsl"`, "shaders/b.glsl", true},
		{`  #  include   "lib/b.glsl"  `, "shaders/lib/b.glsl", true},
		{`#include <lib/b.glsl>`, "lib/b.glsl", true},
		{`#include "b.glsl" // shared helpers`, "shaders/b.glsl", true},
		{"#include \"b.glsl\"\r", "shaders/b.glsl", true},
		{`#include "../b.glsl"`, "b.glsl", true},
		{`// #include "b.glsl"`, "", false},
		{`#includes "b.glsl"`, "", false},
		{`#include`, "", false},
		{`uniform vec2 tileSize;`, "", false},
	}

	for _, tt := range tests {
		target, ok := ParseIncludeDirective(tt.line, "shaders/a.fs.glsl")
		if ok != tt.ok || target != tt.target {
			t.Errorf("ParseIncludeDirective(%q) = (%q, %v), want (%q, %v)", tt.line, target, ok, tt.target, tt.ok)
		}
	}
}

func TestExpandIncludesReportsDirectIncludes(t *testing.T) {
	fsys := fstest.MapFS{
		"shaders/map.fs.glsl":    &fstest.MapFile{Data: []byte("#version 330\n#include \"lib/color.glsl\"\nvoid main() {}\n")},
		"shaders/lib/color.glsl": &fstest.MapFile{Data: []byte("vec3 gray() { return vec3(0.5); }\n")},
	}

	src, includes, err := ExpandIncludes(fsys, "shaders/map.fs.glsl")
	if err != nil {
		t.Fatalf("ExpandIncludes: %v", err)
	}

	want := "#version 330\nvec3 gray() { return vec3(0.5); }\nvoid main() {}\n"
	if got := string(src); got != want {
		t.Errorf("expanded source:\ngot:  %q\nwant: %q", got, want)
	}

	if got, want := strings.Join(includes, " "), "shaders/lib/color.glsl"; got != want {
		t.Errorf("includes: got %q, want %q", got, want)
	}
}

func TestExpandIncludesReportsTransitiveIncludesInExpansionOrder(t *testing.T) {
	fsys := fstest.MapFS{
		"a.glsl": &fstest.MapFile{Data: []byte("#include \"b.glsl\"\n#include <d.glsl>\nA\n")},
		"b.glsl": &fstest.MapFile{Data: []byte("#include \"c.glsl\"\nB\n")},
		"c.glsl": &fstest.MapFile{Data: []byte("C\n")},
		"d.glsl": &fstest.MapFile{Data: []byte("#include \"c.glsl\"\nD\n")},
	}

	src, includes, err := ExpandIncludes(fsys, "a.glsl")
	if err != nil {
		t.Fatalf("ExpandIncludes: %v", err)
	}

	if got, want := string(src), "C\nB\nD\nA\n"; got != want {
		t.Errorf("expanded source: got %q, want %q", got, want)
	}

	if got, want := strings.Join(includes, " "), "b.glsl c.glsl d.glsl"; got != want {
		t.Errorf("includes: got %q, want %q", got, want)
	}
}
