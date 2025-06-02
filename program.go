package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/kballard/go-shellquote"
	"golang.org/x/term"
)

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return err == nil, nil
	}

	return false, fmt.Errorf("checking existence of %s: %w", path, err)
}

func isTerminal(f any) bool {
	if f, ok := f.(interface{ Fd() uintptr }); ok {
		return term.IsTerminal(int(f.Fd()))
	}

	return false
}

type commandError struct {
	status int
}

func (e *commandError) Error() string {
	return fmt.Sprintf("command failed with status %d", e.status)
}

type program struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	dockerCliProgram string

	xdgDBusProxyProgram      string
	xdgDBusProxyReadyTimeout time.Duration

	containerName   string
	image           string
	user            string
	group           string
	mounts          *mountSet
	workdir         string
	envFiles        []string
	env             []string
	shell           string
	args            []string
	interactive     bool
	forwardSSHAgent bool
	forwardDBus     bool
	forwardLocale   bool
}

func newProgram() *program {
	return &program{
		stdin:           os.Stdin,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
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
			return err
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
	p.interactive = isTerminal(p.stdin) && isTerminal(p.stdout) && isTerminal(p.stderr)

	if err := applyDefaultMounts(p.mounts); err != nil {
		return nil
	}

	p.mounts.set(workdir, mountReadWrite)

	return nil
}

func (p *program) registerFlags(app *kingpin.Application) {
	app.Help = `Run command or shell within a container while preserving most of the local execution environment.`

	app.Flag("docker-cli-program", "Name of Docker CLI program or an absolute path.").
		Envar("COCOON_DOCKER_CLI_PROGRAM").
		Default("docker").
		StringVar(&p.dockerCliProgram)

	app.Flag("xdg-dbus-proxy-program", "Name of xdg-dbus-proxy program or an absolute path.").
		Envar("COCOON_XDG_DBUS_PROXY_PROGRAM").
		Default("xdg-dbus-proxy").
		StringVar(&p.xdgDBusProxyProgram)

	app.Flag("xdg-dbus-proxy-ready-timeout", "Amount of time to wait for xdg-dbus-proxy to become ready.").
		Hidden().
		Default("10s").
		DurationVar(&p.xdgDBusProxyReadyTimeout)

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
		`Allocate a pseudo-TTY for the container and enable interactive I/O. Defaults to enabled if standard output is a TTY.`).
		Envar("COCOON_INTERACTIVE").
		BoolVar(&p.interactive)

	app.Flag("forward-ssh-agent", "Expose local SSH agent to container. Enabled by default.").
		Envar("COCOON_FORWARD_SSH_AGENT").
		BoolVar(&p.forwardSSHAgent)

	app.Flag("forward-dbus", "Expose local D-Bus container.").
		Envar("COCOON_FORWARD_DBUS").
		BoolVar(&p.forwardDBus)

	app.Flag("forward-locale", "Set LC_* environment variables in container.").
		Envar("COCOON_FORWARD_LOCALE").
		BoolVar(&p.forwardLocale)

	app.Arg("command", "Command and its arguments. If omitted a shell is started.").
		StringsVar(&p.args)
}

type runtime struct {
	baseDir string
}

func (r *runtime) cleanup() error {
	var err error

	if r.baseDir != "" {
		err = errors.Join(err, os.RemoveAll(r.baseDir))
	}

	return nil
}

func (r *runtime) ensureBaseDir() (string, error) {
	if r.baseDir == "" {
		path, err := os.MkdirTemp("", "tmp-cocoon-*")
		if err != nil {
			return "", err
		}

		r.baseDir = path
	}

	return r.baseDir, nil
}

func (r *runtime) createFile(pattern string) (*os.File, error) {
	baseDir, err := r.ensureBaseDir()
	if err != nil {
		return nil, err
	}

	return os.CreateTemp(baseDir, pattern)
}

func (r *runtime) createDir(pattern string) (string, error) {
	baseDir, err := r.ensureBaseDir()
	if err != nil {
		return "", err
	}

	return os.MkdirTemp(baseDir, pattern)
}

func createTempEnvFile(r *runtime, environ envMap) (_ string, err error) {
	if len(environ) == 0 {
		return "", nil
	}

	f, err := r.createFile("env")
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	if err := writeDockerEnviron(f, environ); err != nil {
		return "", fmt.Errorf("writing environment to %q: %w", f.Name(), err)
	}

	return f.Name(), nil
}

func (p *program) run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	r := &runtime{}

	defer func() {
		if cleanupErr := r.cleanup(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup: %w", cleanupErr))
		}
	}()

	baseEnv := envMap{
		"HOME": nil,
	}

	if p.interactive {
		baseEnv["debian_chroot"] = &p.containerName
	}

	mounts := p.mounts.clone()

	if p.forwardSSHAgent {
		if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
			mounts.set(sshAuthSock, mountReadOnly)
			baseEnv["SSH_AUTH_SOCK"] = &sshAuthSock
		}
	}

	if p.forwardDBus {
		dbusSocket, dbusCleanup, err := p.startDBusProxy(ctx, r)
		if err != nil {
			return fmt.Errorf("D-Bus: %w", err)
		}

		defer func() {
			if dbusErr := dbusCleanup(); dbusErr != nil {
				err = errors.Join(err, fmt.Errorf("D-Bus proxy: %w", dbusErr))
			}
		}()

		mounts.set(dbusSocket, mountReadOnly)
		baseEnv[dbusSessionBusAddressEnv] = &dbusSocket
	}

	if p.forwardLocale {
		for _, i := range localeEnvVariables {
			baseEnv[i] = nil
		}
	}

	env, err := combineEnviron(baseEnv, p.envFiles, p.env)
	if err != nil {
		return err
	}

	envFile, err := createTempEnvFile(r, env)
	if err != nil {
		return err
	}

	args, err := p.toDockerCommand(envFile, mounts)
	if err != nil {
		return err
	}

	if p.interactive {
		log.Printf("Container command: %s", shellquote.Join(args...))
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = p.stdin
	cmd.Stdout = p.stdout
	cmd.Stderr = p.stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true,
		Foreground: isTerminal(p.stdin),
	}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError

		if errors.As(err, &exitErr) && !slices.Contains(dockerExitCodes, exitErr.ExitCode()) {
			return &commandError{status: exitErr.ExitCode()}
		}

		return fmt.Errorf("container runtime: %w", err)
	}

	return nil
}
