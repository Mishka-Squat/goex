package stringsex

import (
	"iter"
	"math"
	"slices"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func Title(s string) string {
	return cases.Title(language.English, cases.NoLower).String(s)
}

func SplitSeq(str string, sep string) iter.Seq[string] {
	return slices.Values(strings.Split(str, sep))
}

func JoinSeq(x iter.Seq[string], sep string) string {
	return strings.Join(slices.Collect(x), sep)
}

func CompactReplace(old string, items string, replace rune) string {
	const IndexMask = math.MaxInt >> 1
	const StateMask = ^IndexMask

	ret := make([]rune, len(old))
	i := 0
	for si, s := range old {
		ret[si-i&IndexMask] = s
		if strings.ContainsRune(items, s) {
			if i&StateMask == 0 { // first item
				i = 1<<62 | i&IndexMask
				ret[si-i&IndexMask] = replace
			} else {
				state := (i & StateMask) >> 62
				switch state {
				case 1:
					i++
				}
			}
		} else {
			i = i & IndexMask
		}
	}
	return string(ret[:len(old)-i])
}
