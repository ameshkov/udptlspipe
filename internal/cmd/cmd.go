// Package cmd is the entry point of the tool.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/AdguardTeam/golibs/log"
	"github.com/ameshkov/udptlspipe/internal/version"
	goFlags "github.com/jessevdk/go-flags"
)

// Main is the entry point for the command-line tool.
func Main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("udptlspipe version: %s\n", version.Version())

		os.Exit(0)
	}

	o, err := parseOptions()
	var flagErr *goFlags.Error
	if errors.As(err, &flagErr) && flagErr.Type == goFlags.ErrHelp {
		// This is a special case when we exit process here as we received
		// --help.
		os.Exit(0)
	}

	if err != nil {
		log.Error("Failed to parse args: %v", err)

		os.Exit(1)
	}

	if o.Verbose {
		log.SetLevel(log.DEBUG)
	}
}
