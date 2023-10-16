package zammadbridge

import (
	"io"
	"log"
	"os"
)

var (
	StdErr     = log.New(os.Stderr, "", log.LstdFlags)
	StdOut     = log.New(os.Stdout, "", log.LstdFlags)
	StdVerbose = log.New(io.Discard, "", log.LstdFlags)
)

// EnableVerboseLogging enables verbose debug statements to Stdout
func EnableVerboseLogging() {
	StdVerbose = log.New(os.Stdout, "[VERBOSE] ", log.LstdFlags)
}
