package main

import (
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hansmi/cocoon/internal/ref"
)

func TestToDockerEnviron(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     envMap
		want    string
		wantErr error
	}{
		{
			name: "empty",
		},
		{
			name: "variables",
			env: envMap{
				"foo":      ref.Ref("bar"),
				"zabc":     ref.Ref("hello world 123"),
				"unsetvar": nil,
			},
			want: ("" +
				"foo=bar\n" +
				"unsetvar\n" +
				"zabc=hello world 123\n"),
		},
		{
			name: "newline",
			env: envMap{
				"a":  ref.Ref("b"),
				"nl": ref.Ref("\n"),
			},
			wantErr: errDockerEnvironNewline,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := toDockerEnviron(tc.env)

			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("toDockerEnviron() error diff (-want +got):\n%s", diff)
			}

			if err == nil {
				t.Cleanup(func() {
					if err := got.file.Close(); err != nil {
						t.Errorf("Closing environment file failed: %v", err)
					}
				})

				if _, err := got.file.Seek(0, os.SEEK_SET); err != nil {
					t.Errorf("Seek() failed: %v", err)
				}

				fileContent, err := io.ReadAll(got.file)
				if err != nil {
					t.Errorf("Reading environment file failed: %v", err)
				}

				if diff := cmp.Diff(tc.want, string(fileContent), cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("Environment file diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}
