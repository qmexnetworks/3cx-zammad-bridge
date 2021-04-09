package zammadbridge

import (
	"log"
	"os"
)

var (
	StdErr = log.New(os.Stderr, "", log.LstdFlags)
	StdOut = log.New(os.Stdout, "", log.LstdFlags)
)
