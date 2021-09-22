package main

import (
	"encoding/base64"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

type arguments struct {
	connections uint32
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

	var c uint64

	flag.Uint64Var(&c, "c", 0, "number of concurrent connections")
	flag.BoolVar(&args.help, "h", false, "displays available flags")
	flag.BoolVar(&args.verbose, "v", false, "enables debug logs")
	flag.StringVar(&args.outputFile, "o", "", "output path of downloaded file, default is same directory.")
	flag.Parse()

	args.connections = uint32(c)

}

func encode(s string, w io.Writer) error {

	enc := base64.NewEncoder(base64.StdEncoding, w)
	defer enc.Close()
	_, err := enc.Write([]byte(s))
	if err != nil {
		return err
	}

	return nil

}

func decode(s string) (string, error) {

	r := strings.NewReader(s)
	out := make([]byte, len(s))
	dec := base64.NewDecoder(base64.StdEncoding, r)
	n, err := dec.Read(out)
	if err != nil {
		return "", err
	}

	return string(out[:n]), nil
}

func parseUint32(s ...string) ([]uint32, error) {

	var err error
	var r uint64
	var ret []uint32

	for _, v := range s {
		r, err = strconv.ParseUint(v, 10, 32)
		if err != nil {
			log.Println(err)
			return ret, err
		}
		ret = append(ret, uint32(r))
	}

	return ret, nil
}
