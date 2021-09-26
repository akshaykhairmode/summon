package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"
)

type arguments struct {
	connections int64
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

	flag.Int64Var(&args.connections, "c", 0, "number of concurrent connections")
	flag.BoolVar(&args.help, "h", false, "displays available flags")
	flag.BoolVar(&args.verbose, "v", false, "enables debug logs")
	flag.StringVar(&args.outputFile, "o", "", "output path of downloaded file, default is same directory.")
	flag.Parse()

}

func encode(b []byte, w io.Writer) error {

	enc := base64.NewEncoder(base64.StdEncoding, w)
	defer enc.Close()
	_, err := enc.Write(b)
	if err != nil {
		return err
	}

	return nil

}

func decode(b []byte) ([]byte, error) {

	r := bytes.NewReader(b)
	out := make([]byte, len(b))
	dec := base64.NewDecoder(base64.StdEncoding, r)
	n, err := dec.Read(out)
	if err != nil {
		return out, err
	}

	return out[:n], nil
}

func parseint64(s ...string) ([]int64, error) {

	var err error
	var r uint64
	var ret []int64

	for _, v := range s {
		r, err = strconv.ParseUint(v, 10, 32)
		if err != nil {
			log.Println(err)
			return ret, err
		}
		ret = append(ret, int64(r))
	}

	return ret, nil
}

func startTimer(s string, args ...interface{}) func() {
	startTime := time.Now()
	str := fmt.Sprintf(s, args...)
	return func() {
		LogWriter.Printf(str+" %v", time.Since(startTime))
	}
}
