package main

import (
	"slices"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSortPaths(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "empty"},
		{
			name: "sort",
			paths: []string{
				"/home/xyz/config",
				"/",
				"/usr/bin",
				"/home/xyz",
				"/home",
			},
			want: []string{
				"/",
				"/home",
				"/home/xyz",
				"/usr/bin",
				"/home/xyz/config",
			},
		},
		{
			name: "multiple",
			paths: []string{
				"/var",
				"/tmp/a",
				"/",
				"/tmp/b/c",
				"/",
				"/var",
			},
			want: []string{
				"/",
				"/",
				"/var",
				"/var",
				"/tmp/a",
				"/tmp/b/c",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := slices.Clone(tc.paths)

			sortPaths(paths)

			if diff := cmp.Diff(tc.want, paths, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("sortPaths() diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMountSet(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{name: "empty"},
		{
			name: "simple",
			args: []string{
				"--ro=/",
				"--rw=/home/foo",
			},
			want: []string{
				"--mount=type=bind,src=/,dst=/,readonly",
				"--mount=type=bind,src=/home/foo,dst=/home/foo",
			},
		},
		{
			name: "override",
			args: []string{
				"--ro=/",
				"--ro=/home/foo",
				"--rw=/etc",
				"--rw=/",
			},
			want: []string{
				"--mount=type=bind,src=/,dst=/",
				"--mount=type=bind,src=/etc,dst=/etc",
				"--mount=type=bind,src=/home/foo,dst=/home/foo,readonly",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newMountSet()

			app := kingpin.New(tc.name, "")

			mountSetVar(app.Flag("ro", ""), s, mountReadOnly)
			mountSetVar(app.Flag("rw", ""), s, mountReadWrite)

			if _, err := app.Parse(tc.args); err != nil {
				t.Errorf("Parsing flags failed: %v", err)
			}

			if diff := cmp.Diff(tc.want, s.toDockerFlags(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Docker flags diff (-want +got):\n%s", diff)
			}
		})
	}
}
