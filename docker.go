package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
)

var errDockerEnvironNewline = errors.New("newline characters not supported in Docker environment variables")

// Exit codes from Docker itself.
//
// https://docs.docker.com/engine/containers/run/#exit-status
// https://tldp.org/LDP/abs/html/exitcodes.html
var dockerExitCodes = []int{
	// Docker client failed.
	125,
	// Container command can't be invoked.
	126,
	// Container command can't be found.
	127,
}

func writeDockerEnvironVariable(w io.Writer, variable string, value *string) error {
	if value != nil && strings.ContainsAny(*value, "\r\n") {
		return errDockerEnvironNewline
	}

	var err error

	if value == nil {
		// Pass-through variable
		_, err = fmt.Fprintf(w, "%s\n", variable)
	} else {
		_, err = fmt.Fprintf(w, "%s=%s\n", variable, *value)
	}

	return err
}

func writeDockerEnviron(w io.Writer, environ envMap) error {
	variables := maps.Keys(environ)

	sort.Strings(variables)

	for _, variable := range variables {
		if err := writeDockerEnvironVariable(w, variable, environ[variable]); err != nil {
			return fmt.Errorf("%s: %w", variable, err)
		}
	}

	return nil
}

func (p *program) toDockerCommand(envFile string, mounts *mountSet) (_ []string, err error) {
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

	args = append(args, mounts.toDockerFlags()...)

	if p.interactive {
		args = append(args, "--interactive", "--tty")
	}

	if envFile != "" {
		args = append(args, fmt.Sprintf("--env-file=%s", envFile))
	}

	args = append(args, p.image)

	if len(p.args) > 1 {
		args = append(args, p.args[1:]...)
	}

	return args, nil
}
