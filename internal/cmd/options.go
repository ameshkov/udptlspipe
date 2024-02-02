package cmd

import (
	"fmt"
	"os"

	goFlags "github.com/jessevdk/go-flags"
)

// Options represents command-line arguments.
type Options struct {
	// ServerMode controls whether the tool works in the server mode.
	// By default, the tool will work in the client mode.
	ServerMode bool `short:"s" long:"pipe" description:"Enables pipe mode." optional:"yes" optional-value:"true"`

	// ListenAddr is the address the tool will be listening to. If it's in the
	// pipe mode, it will listen to tcp://, if it's in the client mode, it
	// will listen to udp://.
	ListenAddr string `short:"l" long:"listen" description:"Address the tool will be listening to." value-name:"<IP>:<Port>"`

	// DestinationAddr is the address the tool will connect to. Depending on the
	// mode (pipe or client) this address has different semantics. In the
	// client mode this is the address of the udptlspipe pipe. In the pipe
	// mode this is the address where the received traffic will be passed.
	DestinationAddr string `short:"d" long:"destination" description:"Address the tool will connect to." value-name:"<IP>:<Port>"`

	// Verbose defines whether we should write the DEBUG-level log or not.
	Verbose bool `short:"v" long:"verbose" description:"Verbose output (optional)." optional:"yes" optional-value:"true"`
}

// parseOptions parses os.Args and creates the Options struct.
func parseOptions() (o *Options, err error) {
	opts := &Options{}
	parser := goFlags.NewParser(opts, goFlags.Default|goFlags.IgnoreUnknown)
	remainingArgs, err := parser.ParseArgs(os.Args[1:])
	if err != nil {
		return nil, err
	}

	if len(remainingArgs) > 0 {
		return nil, fmt.Errorf("unknown arguments: %v", remainingArgs)
	}

	return opts, nil
}
