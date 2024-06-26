package main

import (
	"log"

	"github.com/alecthomas/kingpin/v2"
)

func main() {
	kingpin.CommandLine.Interspersed(false)

	p := newProgram()

	err := p.detectDefaults()

	if err == nil {
		p.registerFlags(kingpin.CommandLine)
		kingpin.Parse()

		err = p.run()
	}

	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
