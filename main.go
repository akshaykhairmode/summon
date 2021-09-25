package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	MAX_CONN      = 20
	DEFAULT_CONN  = 4
	PROGRESS_SIZE = 30
)

func init() {
	log.SetOutput(os.Stdout)
	flag.CommandLine.SetOutput(os.Stdout)
}

var ErrGracefulShutdown = errors.New("Got Stop Signal")

func main() {

	defer recoverMain()

	sum, err := NewSummon()
	if err != nil {
		log.Fatalf("ERROR : %s", err)
	}

	//get the user kill signals
	go sum.catchSignals()

	if err := sum.run(); err != nil {
		log.Fatalf("ERROR : %s", err)
	}

	LogWriter.Printf("Time took : %v", time.Since(sum.startTime))

}

//run is basically the start method
func (sum *summon) run() error {

	isSupported, contentLength, err := getRangeDetails(sum.uri)
	if err != nil {
		return err
	}

	sum.fileDetails.contentLength = contentLength
	sum.isRangeSupported = isSupported

	if !isSupported && !sum.isResume {
		sum.concurrency = 1
	}

	log.Printf("Multiple Connections Supported : %v", isSupported)
	log.Printf("Got Content Length : %v", contentLength)
	log.Printf("Using %v connections", sum.concurrency)

	err = sum.process()

	if err == nil {
		LogWriter.Printf("Success, Now Cleaning Up")
		return sum.deleteFiles(sum.fileDetails.chunks, sum.getMetaFileName())
	}

	//if there was some error we will delete the files except unless its gracefully stopped
	if err != ErrGracefulShutdown {
		LogWriter.Printf("Some error occured Cleaning Up, Error : %v", err)
		return sum.deleteFiles(sum.fileDetails.chunks, sum.fileDetails.tempOutFile.Name(), sum.getMetaFileName())
	}

	return nil

}

func (sum *summon) catchSignals() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigc
		for i := 0; i < len(sum.fileDetails.chunks); i++ {
			sum.stop <- fmt.Errorf("got stop signal : %v", s)
		}
	}()
}

func recoverMain() {
	if err := recover(); err != nil {
		LogWriter.Printf("Recovered Error : %v", err)
	}
}
