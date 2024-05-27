package main

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hansmi/cocoon/internal/testutil"
)

func TestFileExists(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		want    bool
		wantErr error
	}{
		{
			name: "directory",
			path: t.TempDir(),
			want: true,
		},
		{
			name: "file",
			path: testutil.MustWriteFile(t, filepath.Join(t.TempDir(), "empty"), ""),
			want: true,
		},
		{
			name: "missing",
			path: filepath.Join(t.TempDir(), "missing"),
		},
		{
			name:    "not a directory",
			path:    filepath.Join(testutil.MustWriteFile(t, filepath.Join(t.TempDir(), "empty"), ""), "xyz"),
			wantErr: cmpopts.AnyError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fileExists(tc.path)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("fileExists() diff (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("fileExists() error diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProgramDetectDefaults(t *testing.T) {
	p := newProgram()

	if err := p.detectDefaults(); err != nil {
		t.Errorf("detectDefaults() failed: %v", err)
	}
}
