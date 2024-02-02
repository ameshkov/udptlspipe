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
	ServerMode bool `short:"s" long:"server" description:"Enables the server mode. By default it runs in client mode." optional:"yes" optional-value:"true"`

	// ListenAddr is the address the tool will be listening to. If it's in the
	// pipe mode, it will listen to tcp://, if it's in the client mode, it
	// will listen to udp://.
	ListenAddr string `short:"l" long:"listen" description:"Address the tool will be listening to (required)." value-name:"<IP:Port>" required:"true"`

	// DestinationAddr is the address the tool will connect to. Depending on the
	// mode (pipe or client) this address has different semantics. In the
	// client mode this is the address of the udptlspipe pipe. In the pipe
	// mode this is the address where the received traffic will be passed.
	DestinationAddr string `short:"d" long:"destination" description:"Address the tool will connect to (required)." value-name:"<IP:Port>" required:"true"`

	// Password is used to detect if the client is actually allowed to use
	// udptlspipe. If it's not allowed, the server returns a stub web page.
	Password string `short:"p" long:"password" description:"Password is used to detect if the client is allowed." value-name:"<password>"`

	// ProxyURL is the proxy address that should be used when connecting to the
	// destination address.
	ProxyURL string `short:"x" long:"proxy" description:"URL of a proxy to use when connecting to the destination address (optional)." value-name:"[protocol://username:password@]host[:port]"`

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
