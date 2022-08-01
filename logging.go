package zammadbridge

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	StdErr     = log.New(os.Stderr, "", log.LstdFlags)
	StdOut     = log.New(os.Stdout, "", log.LstdFlags)
	StdVerbose = log.New(ioutil.Discard, "", log.LstdFlags)
)

// EnableVerboseLogging enables verbose debug statements to Stdout
func EnableVerboseLogging() {
	StdVerbose = log.New(os.Stdout, "VERBOSE", log.LstdFlags)
}
