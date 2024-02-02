// Package cmd is the entry point of the tool.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AdguardTeam/golibs/log"
	"github.com/ameshkov/udptlspipe/internal/pipe"
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

	srv, err := pipe.NewServer(o.ListenAddr, o.DestinationAddr, o.Password, o.ProxyURL, o.ServerMode)
	if err != nil {
		log.Error("Failed to initialize server: %v", err)

		os.Exit(1)
	}

	err = srv.Start()
	if err != nil {
		log.Error("Failed to start the server: %v", err)

		os.Exit(1)
	}

	// Subscribe to the OS events.
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

	// Wait until the user stops the tool.
	<-signalChannel

	// Gracefully shutdown the server.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*10))
	defer cancel()
	err = srv.Shutdown(ctx)
	if err != nil {
		log.Info("Failed to gracefully shutdown the server: %v", err)
	}

	log.Info("Exiting udptlspipe.")
}
