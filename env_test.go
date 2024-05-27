package main

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hansmi/cocoon/internal/ref"
	"github.com/hansmi/cocoon/internal/testutil"
)

func TestCombineEnviron(t *testing.T) {
	for _, tc := range []struct {
		name    string
		base    envMap
		files   []string
		literal []string
		want    envMap
		wantErr error
	}{
		{name: "empty"},
		{
			name: "files",
			files: []string{
				testutil.MustWriteFile(t, filepath.Join(t.TempDir(), "empty"), ""),
				testutil.MustWriteFile(t, filepath.Join(t.TempDir(), "yaml"), "# comment\nfirst: 1\nkey: value\n"),
				testutil.MustWriteFile(t, filepath.Join(t.TempDir(), "yaml"), "# comment\nfirst: 2\nunset: ~"),
			},
			want: envMap{
				"key":   ref.Ref("value"),
				"first": ref.Ref("2"),
				"unset": nil,
			},
		},
		{
			name: "file not found",
			files: []string{
				filepath.Join(t.TempDir(), "missing"),
			},
			wantErr: fs.ErrNotExist,
		},
		{
			name: "literals",
			base: envMap{
				"x": ref.Ref("hello"),
				"y": nil,
			},
			literal: []string{
				"a=1",
				"b",
				"c=3",
			},
			want: envMap{
				"a": ref.Ref("1"),
				"b": nil,
				"c": ref.Ref("3"),
				"x": ref.Ref("hello"),
				"y": nil,
			},
		},
		{
			name: "mixed",
			base: envMap{
				"base": ref.Ref("a"),
			},
			files: []string{
				testutil.MustWriteFile(t, filepath.Join(t.TempDir(), "empty"), ""),
				testutil.MustWriteFile(t, filepath.Join(t.TempDir(), "yaml"), "---\nfile: b\n"),
			},
			literal: []string{
				"literal=c",
			},
			want: envMap{
				"base":    ref.Ref("a"),
				"file":    ref.Ref("b"),
				"literal": ref.Ref("c"),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := combineEnviron(tc.base, tc.files, tc.literal)

			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("combineEnviron() error diff (-want +got):\n%s", diff)
			}

			if err == nil {
				if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("combineEnviron() result diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}
