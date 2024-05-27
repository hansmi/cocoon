package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func clearCloseOnExec(fd uintptr) error {
	flags, err := unix.FcntlInt(fd, unix.F_GETFD, 0)
	if err != nil {
		return fmt.Errorf("getting file descriptor flags: %w", err)
	}

	if (flags & unix.FD_CLOEXEC) != 0 {
		_, err = unix.FcntlInt(fd, unix.F_SETFD, flags & ^unix.FD_CLOEXEC)
		if err != nil {
			return fmt.Errorf("clearing close-on-exec file descriptor flag: %w", err)
		}
	}

	return nil
}
