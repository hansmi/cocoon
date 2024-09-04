package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const dbusSessionBusAddressEnv = "DBUS_SESSION_BUS_ADDRESS"

func (p *program) startDBusProxy(ctx context.Context, r *runtime) (string, func() error, error) {
	sessionBusAddress := os.Getenv(dbusSessionBusAddressEnv)
	if sessionBusAddress == "" {
		return "", nil, fmt.Errorf("environment variable %q is unset or empty", dbusSessionBusAddressEnv)
	}

	sockDir, err := r.createDir("dbus")
	if err != nil {
		return "", nil, err
	}

	sock := filepath.Join(sockDir, "socket")

	pr, pw, err := os.Pipe()
	if err != nil {
		return "", nil, err
	}

	cmd := exec.CommandContext(ctx, p.xdgDBusProxyProgram, "--fd=3", sessionBusAddress, sock)
	cmd.Stdout = p.stderr
	cmd.Stderr = p.stderr
	cmd.ExtraFiles = append(cmd.ExtraFiles, pw)

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("launching proxy: %w", err)
	}

	if err := pw.Close(); err != nil {
		return "", nil, fmt.Errorf("closing pipe write end: %w", err)
	}

	// The proxy is now on its own. It'll terminate automatically when the
	// pipe's reading side is closed.

	cmdErrCh := make(chan error, 1)

	go func() {
		defer close(cmdErrCh)

		cmdErrCh <- cmd.Wait()
	}()

	readyErrCh := make(chan error, 1)

	go func() {
		defer close(readyErrCh)

		deadline := time.Now().Add(p.xdgDBusProxyReadyTimeout)

		if err := pr.SetReadDeadline(deadline); err != nil {
			readyErrCh <- err
			return
		}

		buf := make([]byte, 1)

		if n, err := pr.Read(buf); err != nil {
			readyErrCh <- fmt.Errorf("reading notification pipe: %w", err)
		} else if n == 0 {
			readyErrCh <- errors.New("notification pipe closed prematurely")
		}
	}()

	// Wait for proxy to either terminate or signal its readiness.
	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()

	case err := <-cmdErrCh:
		if err == nil {
			err = errors.New("success")
		}

		return "", nil, fmt.Errorf("proxy exited before becoming ready: %w", err)

	case err := <-readyErrCh:
		if err != nil {
			return "", nil, err
		}
	}

	return sock, func() error {
		if err := pr.Close(); err != nil {
			return fmt.Errorf("closing pipe read end: %w", err)
		}

		// Wait for the proxy to terminate.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-cmdErrCh:
			return err
		}
	}, nil
}
