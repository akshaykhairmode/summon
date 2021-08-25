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
)

type summon struct {
	concurrency int
	uri         string
	chunks      map[int][]byte
	err         error
	opath       string
	*sync.Mutex
}

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

	if err := sum.run(); err != nil {
		log.Fatalf("ERROR : %s", err)
	}

}

func NewSummon() (*summon, error) {

	c := flag.Int("c", 0, "number of concurrent connections")
	h := flag.Bool("h", false, "displays available flags")
	o := flag.String("o", "", "output path of downloaded file, default is same directory.")
	flag.Parse()

	if *h {
		flag.PrintDefaults()
		fmt.Println("\nExample Usage - $GOBIN/summon -c 5 http://www.africau.edu/images/default/sample.pdf")
		return nil, nil
	}

	if *c <= 0 {
		*c = 1
	}

	if len(flag.Args()) <= 0 {
		return nil, fmt.Errorf("Please pass file url")
	}

	u := flag.Args()[0]
	uri, err := url.ParseRequestURI(u)
	if err != nil {
		return nil, fmt.Errorf("Passed URL is invalid")
	}

	sum := new(summon)
	sum.concurrency = *c
	sum.uri = uri.String()
	sum.chunks = make(map[int][]byte)
	sum.Mutex = &sync.Mutex{}

	if *o != "" {
		sum.opath = *o
	}

	return sum, nil

}

func (sum *summon) run() error {

	isSupported, contentLength, err := getRangeDetails(sum.uri)

	if err != nil {
		return err
	}

	if !isSupported || sum.concurrency <= 1 {
		return sum.processSingle()
	}

	return sum.processMultiple(contentLength)

}

func (sum *summon) processSingle() error {

	//Initialize first index with []byte
	sum.chunks[0] = make([]byte, 0)
	sum.downloadFileForRange(nil, sum.uri, "", 0)

	if sum.err != nil {
		return sum.err
	}

	return sum.combineChunks()
}

func (sum *summon) processMultiple(contentLength int) error {

	split := contentLength / sum.concurrency

	wg := &sync.WaitGroup{}
	index := 0

	for i := 0; i < contentLength; i += split + 1 {
		j := i + split
		if j > contentLength {
			j = contentLength
		}

		//Initialize for each index or application will panic
		sum.chunks[index] = make([]byte, 0)
		wg.Add(1)
		go sum.downloadFileForRange(wg, sum.uri, strconv.Itoa(i)+"-"+strconv.Itoa(j), index)
		index++
	}

	wg.Wait()

	if sum.err != nil {
		return sum.err
	}

	return sum.combineChunks()
}

//combineChunks will combine the chunks in ordered fashion starting from 1
func (sum *summon) combineChunks() error {

	var fname string

	if sum.opath != "" {
		fname = sum.opath
	} else {
		currDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("Error while getting pwd : %v", err)
		}
		fname = currDir + "/" + filepath.Base(sum.uri)
	}

	out, err := os.Create(fname)
	defer out.Close()

	if err != nil {
		return fmt.Errorf("Error while creating file : %v", err)
	}

	buf := bytes.NewBuffer(nil)
	//Not using for range because it does not gurantee ordered iteration
	for i := 0; i < len(sum.chunks); i++ {
		buf.Write(sum.chunks[i])
	}

	l, err := buf.WriteTo(out)
	if err != nil {
		return fmt.Errorf("Error while writing to file : %v", err)
	}

	log.Printf("Wrote to File : %v, len : %v", fname, l)

	return nil
}

//downloadFileForRange will download the file for the provided range and set the bytes to the chunk map, will set summor.error field if error occurs
func (sum *summon) downloadFileForRange(wg *sync.WaitGroup, u, r string, index int) {

	if wg != nil {
		defer wg.Done()
	}

	request, err := http.NewRequest("GET", u, strings.NewReader(""))
	if err != nil {
		sum.err = err
		return
	}

	if r != "" {
		request.Header.Add("Range", "bytes="+r)
	}

	sc, _, data, err := doAPICall(request)
	if err != nil {
		sum.err = err
		return
	}

	//206 = Partial Content
	if sc != 200 && sc != 206 {
		sum.Lock()
		sum.err = fmt.Errorf("Did not get 20X status code, got : %v", sc)
		sum.Unlock()
		log.Println(sum.err)
		return
	}

	sum.Lock()
	sum.chunks[index] = append(sum.chunks[index], data...)
	sum.Unlock()
}

//getRangeDetails returns ifRangeIsSupported,statuscode,error
func getRangeDetails(u string) (bool, int, error) {

	request, err := http.NewRequest("HEAD", u, strings.NewReader(""))
	if err != nil {
		return false, 0, fmt.Errorf("Error while creating request : %v", err)
	}

	sc, headers, _, err := doAPICall(request)
	if err != nil {
		return false, 0, fmt.Errorf("Error calling url : %v", err)
	}

	if sc != 200 && sc != 206 {
		return false, 0, fmt.Errorf("Did not get 200 or 206 response")
	}

	conLen := headers.Get("Content-Length")
	cl, err := strconv.Atoi(conLen)
	if err != nil {
		return false, 0, fmt.Errorf("Error Parsing content length : %v", err)
	}

	//Accept-Ranges: bytes
	if headers.Get("Accept-Ranges") == "bytes" {
		return true, cl, nil
	}

	return false, cl, nil

}

//doAPICall will do the api call and return statuscode,headers,data,error respectively
func doAPICall(request *http.Request) (int, http.Header, []byte, error) {

	client := http.Client{
		Timeout: 0,
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, http.Header{}, []byte{}, fmt.Errorf("Error while doing request : %v", err)
	}
	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return 0, http.Header{}, []byte{}, fmt.Errorf("Error while reading response body : %v", err)
	}

	return response.StatusCode, response.Header, data, nil

}
