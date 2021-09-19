package main

import "log"

type Logger interface {
	Printf(format string, args ...interface{})
}

type ProdLogger struct{}

type DevLogger struct{}

//Printf we keep prodlogger empty as we dont want to print anything even if print is called
func (this ProdLogger) Printf(format string, args ...interface{}) {

}

//Printf if verbose option is selected this function will be called so we need to print all of them
func (this DevLogger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

//LogWriter will hold our logger
var LogWriter Logger
