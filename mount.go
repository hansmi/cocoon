package main

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"golang.org/x/exp/maps"
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

// sortPaths reorders a slice such that paths with fewer parts appear earlier.
func sortPaths(paths []string) {
	slices.SortFunc(paths, func(a, b string) int {
		ac, bc := countSeparator(a), countSeparator(b)

		if ac == bc {
			return cmp.Compare(a, b)
		}

		return cmp.Compare(ac, bc)
	})
}

func newMountSet() *mountSet {
	return &mountSet{
		entries: map[string]mountMode{},
	}
}

func (s *mountSet) String() string {
	return fmt.Sprint(s.entries)
}

func (s *mountSet) set(path string, mode mountMode) {
	path = filepath.Clean(path)

	if existing, ok := s.entries[path]; !ok || existing == mountReadOnly {
		s.entries[path] = mode
	}
}

func (s *mountSet) toDockerFlags() []string {
	paths := maps.Keys(s.entries)

	sortPaths(paths)

	var result []string

	for _, path := range paths {
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
