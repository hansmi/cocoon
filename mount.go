package main

import (
	"cmp"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
)

//go:generate stringer -type=mountMode -linecomment -output=mount_string.go
type mountMode int

const (
	mountReadOnly  mountMode = iota // ro
	mountReadWrite                  // rw
)

type mountSet struct {
	entries map[string]mountMode
}

var _ fmt.Stringer = (*mountSet)(nil)

func countSeparator(s string) int {
	return strings.Count(s, string(filepath.Separator))
}

// comparePaths moves the paths with fewer parts appear earlier.
func comparePaths(a, b string) int {
	if ac, bc := countSeparator(a), countSeparator(b); ac != bc {
		return cmp.Compare(ac, bc)
	}

	return cmp.Compare(a, b)
}

func newMountSet() *mountSet {
	return &mountSet{
		entries: map[string]mountMode{},
	}
}

func (s *mountSet) String() string {
	return fmt.Sprint(s.entries)
}

func (s *mountSet) clone() *mountSet {
	result := newMountSet()
	maps.Copy(result.entries, s.entries)
	return result
}

func (s *mountSet) set(path string, mode mountMode) {
	path = filepath.Clean(path)

	if existing, ok := s.entries[path]; !ok || existing == mountReadOnly {
		s.entries[path] = mode
	}
}

func (s *mountSet) toDockerFlags() []string {
	var result []string

	for _, path := range slices.SortedFunc(maps.Keys(s.entries), comparePaths) {
		value := fmt.Sprintf("--mount=type=bind,src=%[1]s,dst=%[1]s", path)

		if s.entries[path] != mountReadWrite {
			value += ",readonly"
		}

		result = append(result, value)
	}

	return result
}

type mountSetFlag struct {
	s    *mountSet
	mode mountMode
}

var _ kingpin.Value = (*mountSetFlag)(nil)

func (f *mountSetFlag) String() string {
	return f.s.String()
}

func (*mountSetFlag) IsCumulative() bool {
	return true
}

func (f *mountSetFlag) Set(value string) error {
	for _, i := range filepath.SplitList(value) {
		f.s.set(i, f.mode)
	}

	return nil
}

func mountSetVar(s kingpin.Settings, target *mountSet, mode mountMode) {
	s.SetValue(&mountSetFlag{
		s:    target,
		mode: mode,
	})
}
