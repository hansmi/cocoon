package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/sys/unix"
)

var errDockerEnvironNewline = errors.New("newline characters not supported in Docker environment variables")

type dockerEnviron struct {
	file *os.File
}

func newDockerEnviron() (*dockerEnviron, error) {
	// Create a temporary file without name.
	fd, err := unix.Open(os.TempDir(), unix.O_TMPFILE|unix.O_RDWR|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("creating temporary file: %w", err)
	}

	return &dockerEnviron{
		file: os.NewFile(uintptr(fd), ""),
	}, nil
}

func (e *dockerEnviron) add(variable string, value *string) error {
	if value != nil && strings.ContainsAny(*value, "\r\n") {
		return errDockerEnvironNewline
	}

	var err error

	if value == nil {
		// Pass-through variable
		_, err = fmt.Fprintf(e.file, "%s\n", variable)
	} else {
		_, err = fmt.Fprintf(e.file, "%s=%s\n", variable, *value)
	}

	return err
}

func toDockerEnviron(environ envMap) (*dockerEnviron, error) {
	result, err := newDockerEnviron()
	if err != nil {
		return nil, err
	}

	variables := maps.Keys(environ)

	sort.Strings(variables)

	for _, variable := range variables {
		if err := result.add(variable, environ[variable]); err != nil {
			return nil, fmt.Errorf("%s: %w", variable, err)
		}
	}

	return result, nil
}

func (p *program) toDockerCommand(environ envMap) ([]string, error) {
	dockerCli, err := exec.LookPath(p.dockerCliProgram)
	if err != nil {
		return nil, fmt.Errorf("unable to find Docker CLI: %w", err)
	}

	entrypoint := p.shell

	if len(p.args) > 0 {
		entrypoint = p.args[0]
	}

	args := []string{
		dockerCli, "run",

		"--entrypoint=" + entrypoint,
		"--init",
		"--name=" + p.containerName,
		"--network=host",
		"--pid=host",
		"--read-only",
		"--rm",
		"--user=" + p.user + ":" + p.group,
		"--uts=host",
		"--workdir=" + p.workdir,

		// TODO: Should there be a "--mount-tmpfs" flag?
		"--tmpfs=/tmp:rw,exec",
	}

	args = append(args, p.mounts.toDockerFlags()...)

	if p.interactive {
		args = append(args, "--interactive", "--tty")
	}

	if len(environ) > 0 {
		env, err := toDockerEnviron(environ)
		if err != nil {
			return nil, err
		}

		if err := clearCloseOnExec(env.file.Fd()); err != nil {
			return nil, err
		}

		args = append(args, fmt.Sprintf("--env-file=/dev/fd/%d", env.file.Fd()))
	}

	args = append(args, p.image)

	if len(p.args) > 1 {
		args = append(args, p.args[1:]...)
	}

	return args, nil
}
