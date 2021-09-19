package main

import (
	"flag"
	"os"
)

type arguments struct {
	connections int
	help        bool
	outputFile  string
	verbose     bool
}

func fileExists(fname string) bool {

	if _, err := os.Stat(fname); !os.IsNotExist(err) {
		return true
	}

	return false

}

func parseFlags(args *arguments) {

	flag.IntVar(&args.connections, "c", 0, "number of concurrent connections")
	flag.BoolVar(&args.help, "h", false, "displays available flags")
	flag.BoolVar(&args.verbose, "v", false, "enables debug logs")
	flag.StringVar(&args.outputFile, "o", "", "output path of downloaded file, default is same directory.")
	flag.Parse()

}
