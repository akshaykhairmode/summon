package main

import "log"

type Logger interface {
	Printf(format string, args ...interface{})
	Println(args ...interface{})
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

//Printf we keep prodlogger empty as we dont want to print anything even if print is called
func (this ProdLogger) Println(args ...interface{}) {

}

//Printf if verbose option is selected this function will be called so we need to print all of them
func (this DevLogger) Println(args ...interface{}) {
	log.Println(args...)
}

//LogWriter will hold our logger
var LogWriter Logger

func setLogWriter(v bool) {

	if v {
		LogWriter = DevLogger{}
		return
	}

	LogWriter = ProdLogger{}
}
