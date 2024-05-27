# Run command in container while preserving local execution environment

[![Latest release](https://img.shields.io/github/v/release/hansmi/cocoon)][releases]
[![CI workflow](https://github.com/hansmi/cocoon/actions/workflows/ci.yaml/badge.svg)](https://github.com/hansmi/cocoon/actions/workflows/ci.yaml)
[![Go reference](https://pkg.go.dev/badge/github.com/hansmi/cocoon.svg)](https://pkg.go.dev/github.com/hansmi/cocoon)

Cocoon is command line program for running a command in a Linux container while
having the most important files and directories from the host system
bind-mounted. Environment variables are configurable as well.


## Installation

[Pre-built binaries][releases]:

* Binary archives (`.tar.gz`)
* Debian/Ubuntu (`.deb`)
* RHEL/Fedora (`.rpm`)

It's also possible to build locally using [Go][golang] or
[GoReleaser][goreleaser].


[golang]: https://golang.org/
[goreleaser]: https://goreleaser.com/
[releases]: https://github.com/hansmi/cocoon/releases/latest

<!-- vim: set sw=2 sts=2 et : -->
