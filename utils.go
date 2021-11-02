package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
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

func getProgressSize() int {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()

	if err != nil {
		LogWriter.Printf("error occured while size command : %v", err)
		return DEFAULT_PROGRESS_SIZE
	}

	data := strings.Split(strings.TrimRight(string(out), "\n"), " ")
	if len(data) < 1 {
		return DEFAULT_PROGRESS_SIZE
	}

	i, err := strconv.Atoi(data[1])
	if err != nil {
		LogWriter.Printf("error occured while converting str to int : %v", err)
		return DEFAULT_PROGRESS_SIZE
	}

	//35 percent of the available terminal size
	return int(math.Round(0.35 * float64(i)))
}

func printWarnings() {
	if runtime.GOOS == "windows" {
		log.Println("WARNING: It may not work as expected on windows")
	}
}

func humanSizeFromBytes(b int64) string {

	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])

}
