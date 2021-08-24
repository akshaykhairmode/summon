package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var c int
var mx = &sync.Mutex{}
var chunk [][]byte

func main() {

	flag.IntVar(&c, "c", 0, "")
	flag.Parse()

	if len(flag.Args()) <= 0 {
		log.Fatalln("URL is empty")
	}

	u := flag.Args()[0]

	uri, err := url.ParseRequestURI(u)
	if err != nil {
		log.Fatalln("Invaid URL")
	}

	if c <= 0 {
		c = 1
	}

	chunk = make([][]byte, c+1)

	isSupported, contentLength := getRangeDetails(u)
	// log.Printf("Content Length is : %v , range support : %v", contentLength, isSupported)

	if !isSupported || c <= 1 {
		chunk[0] = make([]byte, 0)
		downloadFileForRange(nil, u, "", 0)
		final(uri)
		return
	}

	split := contentLength / c

	wg := &sync.WaitGroup{}
	index := 0

	for i := 0; i < contentLength; i += split + 1 {
		j := i + split
		if j > contentLength {
			j = contentLength
		}
		chunk[index] = make([]byte, 0)
		wg.Add(1)
		go downloadFileForRange(wg, u, strconv.Itoa(i)+"-"+strconv.Itoa(j), index)
		index++
	}

	wg.Wait()
	final(uri)

}

func final(uri *url.URL) {

	filename := filepath.Base(uri.String())

	fname := fmt.Sprintf("%v-%v", time.Now().Unix(), filename)

	out, err := os.Create(fname)
	defer out.Close()

	if err != nil {
		log.Fatal(err)
	}

	buf := bytes.NewBuffer(nil)
	for _, v := range chunk {
		buf.Write(v)
	}

	l, err := buf.WriteTo(out)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Wrote to File : %v, len : %v", fname, l)

}

func downloadFileForRange(wg *sync.WaitGroup, u, r string, index int) {

	if wg != nil {
		defer wg.Done()
	}

	request, err := http.NewRequest("GET", u, strings.NewReader(""))
	if err != nil {
		log.Fatalln(err)
	}

	if r != "" {
		request.Header.Add("Range", "bytes="+r)
	}

	sc, _, data := doAPICall(request)

	if sc != 200 && sc != 206 {
		log.Fatalf("Did not get 20X status code, got : %v", sc)
	}

	mx.Lock()
	chunk[index] = append(chunk[index], data...)
	mx.Unlock()
}

func getRangeDetails(u string) (bool, int) {

	request, err := http.NewRequest("HEAD", u, strings.NewReader(""))
	if err != nil {
		log.Fatalln(err)
	}

	sc, headers, _ := doAPICall(request)

	if sc != 200 && sc != 206 {
		log.Fatalln(err)
	}

	conLen := headers.Get("Content-Length")
	cl, err := strconv.Atoi(conLen)
	if err != nil {
		log.Fatal(err)
	}

	//Accept-Ranges: bytes
	if headers.Get("Accept-Ranges") == "bytes" {
		return true, cl
	}

	return false, cl

}

func doAPICall(request *http.Request) (int, http.Header, []byte) {

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	response, err := client.Do(request)
	if err != nil {
		log.Fatalln(err)
	}
	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalln(err)
	}

	return response.StatusCode, response.Header, data

}
