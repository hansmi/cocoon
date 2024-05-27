package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/alecthomas/kingpin/v2"
	"github.com/kballard/go-shellquote"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return err == nil, nil
	}

	return false, err
}

type program struct {
	dockerCliProgram string
	containerName    string
	image            string
	user             string
	group            string
	mounts           *mountSet
	workdir          string
	envFiles         []string
	env              []string
	shell            string
	args             []string
	interactive      bool
	forwardSSHAgent  bool
}

func newProgram() *program {
	return &program{
		mounts:          newMountSet(),
		forwardSSHAgent: true,
	}
}

func applyDefaultMounts(s *mountSet) error {
	for _, path := range []string{
		"/etc/group",
		"/etc/hosts",
		"/etc/localtime",
		"/etc/passwd",
	} {
		s.set(path, mountReadOnly)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	s.set(home, mountReadOnly)

	for path, mode := range map[string]mountMode{
		filepath.Join(home, ".ssh"): mountReadWrite,
	} {
		if ok, err := fileExists(path); err != nil {
			return fmt.Errorf("checking existence of %s: %w", path, err)
		} else if ok {
			s.set(path, mode)
		}
	}

	return nil
}

func (p *program) detectDefaults() error {
	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	p.containerName = fmt.Sprintf("cocoon-%d-%s-%d",
		os.Getuid(), filepath.Base(workdir), os.Getpid())
	p.user = strconv.Itoa(os.Getuid())
	p.group = strconv.Itoa(os.Getgid())
	p.workdir = workdir
	p.shell = "/bin/sh"
	p.interactive = term.IsTerminal(int(os.Stdin.Fd()))

	if err := applyDefaultMounts(p.mounts); err != nil {
		return nil
	}

	p.mounts.set(workdir, mountReadWrite)

	if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		p.mounts.set(sshAuthSock, mountReadOnly)
		p.env = append(p.env, "SSH_AUTH_SOCK="+sshAuthSock)
	}

	return nil
}

func (p *program) registerFlags(app *kingpin.Application) {
	app.Help = `Run command or shell within a container while preserving most of the local execution environment.`

	app.Flag("docker-cli-program", "Name of Docker CLI program or an absolute path.").
		Envar("COCOON_DOCKER_CLI_PROGRAM").
		Default("docker").
		StringVar(&p.dockerCliProgram)

	app.Flag("container-name", "Container name. The default value includes the user ID, directory name and PID.").
		Envar("COCOON_CONTAINER_NAME").
		Default(p.containerName).
		StringVar(&p.containerName)

	app.Flag("image", `OCI image name and an optional tag, e.g. "docker.io/library/alpine:latest"`).
		Envar("COCOON_IMAGE").
		Required().
		StringVar(&p.image)

	app.Flag("user", "User name or ID within the container.").
		Hidden().
		Default(p.user).
		StringVar(&p.user)

	app.Flag("group", "Group name or ID within the container.").
		Hidden().
		Default(p.group).
		StringVar(&p.group)

	mountSetVar(
		app.Flag("mount",
			fmt.Sprintf(`Mount a path into the container in read-only mode. Multiple paths can be specified by passing the flag more than once or by separating paths using %q.`, filepath.ListSeparator)).
			PlaceHolder("PATH").
			Envar("COCOON_MOUNT"),
		p.mounts, mountReadOnly)

	mountSetVar(
		app.Flag("mount-rw", `Mount a path in read-write mode. See "--mount" for additional details.`).
			PlaceHolder("PATH").
			Envar("COCOON_MOUNT_RW"),
		p.mounts, mountReadWrite)

	app.Flag("workdir",
		`Working directory within the container. Defaults to current working directory.`).
		PlaceHolder("DIR").
		Default(p.workdir).
		ExistingDirVar(&p.workdir)

	app.Flag("env-file",
		`Read environment variables from a YAML or JSON file. The content must be a map from string to string or null.`).
		PlaceHolder("FILE").
		Envar("COCOON_ENV_FILE").
		ExistingFilesVar(&p.envFiles)

	app.Flag("env",
		`Set environment variable. Names without "=" mark pass-through variables copied from the local environment if defined.`).
		PlaceHolder("NAME=VALUE").
		StringsVar(&p.env)

	app.Flag("shell", "Shell to run within container when no command is specified.").
		Envar("COCOON_SHELL").
		Default(p.shell).
		StringVar(&p.shell)

	app.Flag("interactive",
		`Allocate a pseudo-TTY for the container and enable interactive I/O. Defaults to enabled if standard input is a TTY.`).
		Envar("COCOON_INTERACTIVE").
		BoolVar(&p.interactive)

	app.Flag("forward-ssh-agent", "Expose local SSH agent to container. Enabled by default.").
		Envar("COCOON_FORWARD_SSH_AGENT").
		BoolVar(&p.forwardSSHAgent)

	app.Arg("command", "Command and its arguments. If omitted a shell is started.").
		StringsVar(&p.args)
}

func (p *program) run() error {
	baseEnv := envMap{
		"HOME": nil,
	}

	if p.interactive {
		baseEnv["debian_chroot"] = &p.containerName
	}

	env, err := combineEnviron(baseEnv, p.envFiles, p.env)
	if err != nil {
		return err
	}

	args, err := p.toDockerCommand(env)
	if err != nil {
		return err
	}

	if p.interactive {
		log.Printf("Container command: %s", shellquote.Join(args...))
	}

	if err := unix.Exec(args[0], args, os.Environ()); err != nil {
		return fmt.Errorf("launching container program: %w", err)
	}

	return nil
}
