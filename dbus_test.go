package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStartDBusProxy(t *testing.T) {
	for _, tc := range []struct {
		program string
	}{
		{program: "/bin/false"},
		{program: "/bin/true"},
	} {
		t.Run(tc.program, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			t.Cleanup(cancel)

			t.Setenv(dbusSessionBusAddressEnv, os.DevNull)

			var r runtime
			var stderr strings.Builder

			p := newProgram()
			p.stderr = &stderr

			p.xdgDBusProxyProgram = tc.program
			p.xdgDBusProxyReadyTimeout = time.Second

			_, _, err := p.startDBusProxy(ctx, &r)

			if err == nil {
				t.Errorf("Expected error, got %v", err)
			}
		})
	}
}
