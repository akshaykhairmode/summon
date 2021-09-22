package main

import (
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

func main() {

	sum, err := NewSummon()
	if err != nil {
		log.Fatalf("ERROR : %s", err)
	}

	if sum == nil {
		return
	}

	//get the user kill signals
	go sum.catchSignals()

	if err := sum.run(); err != nil {
		log.Fatalf("ERROR : %s", err)
	}

	for _, f := range sum.fileDetails.chunks {
		file := f.Name()
		f.Close()
		os.Remove(file)
	}

	log.Printf("Time took : %v", time.Since(sum.startTime))

}

//run is basically the start method
func (sum *summon) run() error {

	isSupported, contentLength, err := getRangeDetails(sum.uri)
	if err != nil {
		return err
	}

	sum.fileDetails.contentLength = contentLength

	if !isSupported && !sum.isResume {
		sum.concurrency = 1
	}

	log.Printf("Multiple Connections Supported : %v", isSupported)
	log.Printf("Got Content Length : %v", contentLength)
	log.Printf("Using %v connections", sum.concurrency)

	return sum.process()

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
