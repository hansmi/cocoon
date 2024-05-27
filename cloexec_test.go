package main

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func getCloseOnExec(t *testing.T, f *os.File) bool {
	t.Helper()

	flags, err := unix.FcntlInt(f.Fd(), unix.F_GETFD, 0)
	if err != nil {
		t.Fatalf("Getting %v file descriptor flags: %v", f, err)
	}

	return flags&unix.FD_CLOEXEC != 0
}

func TestClearCloseOnExec(t *testing.T) {
	tmpdir := t.TempDir()

	f, err := os.CreateTemp(tmpdir, "")
	if err != nil {
		t.Errorf("CreateTemp(%q) failed: %v", tmpdir, err)
	}

	if !getCloseOnExec(t, f) {
		t.Errorf("Expected close-on-exit to be set by default, but it's not on %#v", f)
	}

	for i := 0; i < 3; i++ {
		if err := clearCloseOnExec(f.Fd()); err != nil {
			t.Errorf("clearCloseOnExec() failed: %v", err)
		}

		if getCloseOnExec(t, f) {
			t.Error("Expected close-on-exit to be unset")
		}
	}
}
