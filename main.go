package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/alecthomas/kingpin/v2"
)

func main() {
	kingpin.CommandLine.Interspersed(false)

	p := newProgram()

	err := p.detectDefaults()

	if err == nil {
		p.registerFlags(kingpin.CommandLine)
		kingpin.Parse()

		err = p.run(context.Background())

		var cmdErr *commandError

		if errors.As(err, &cmdErr) {
			os.Exit(cmdErr.status)
		}
	}

	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
