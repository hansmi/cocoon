package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hansmi/cocoon/internal/ref"
)

func TestWriteDockerEnviron(t *testing.T) {
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
			var buf strings.Builder

			err := writeDockerEnviron(&buf, tc.env)

			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("toDockerEnviron() error diff (-want +got):\n%s", diff)
			}

			if err == nil {
				if diff := cmp.Diff(tc.want, buf.String(), cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("Environment file diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}
