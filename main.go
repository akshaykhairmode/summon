package main

import (
	"flag"
	"fmt"
	"io"
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

const (
	MAX_CONN = 60
	MIN_CONN = 1
)

type summon struct {
	concurrency   int              //No. of connections
	uri           string           //URL of the file we want to download
	chunks        map[int]*os.File //Map of temporary files we are creating
	err           error            //used when error occurs inside a goroutine
	startTime     time.Time        //to track time took
	fileName      string           //name of the file we are downloading
	out           *os.File         //output / downloaded file
	*sync.RWMutex                  //mutex to lock the map which accessing it concurrently
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

	log.Printf("Time took : %v", time.Since(sum.startTime))

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

	if len(flag.Args()) <= 0 {
		return nil, fmt.Errorf("Please pass file url")
	}

	u := flag.Args()[0]
	uri, err := url.ParseRequestURI(u)
	if err != nil {
		return nil, fmt.Errorf("Passed URL is invalid")
	}

	sum := new(summon)
	sum.setConcurrency(*c)
	sum.uri = uri.String()
	sum.chunks = make(map[int]*os.File)
	sum.startTime = time.Now()
	sum.fileName = filepath.Base(sum.uri)
	sum.RWMutex = &sync.RWMutex{}

	if err := sum.createOutputFile(*o); err != nil {
		return nil, err
	}

	return sum, nil

}

//setConcurrency set the concurrency as per min and max
func (sum *summon) setConcurrency(c int) {

	if c <= 0 {
		sum.concurrency = MIN_CONN
		return
	}

	if c >= 60 {
		sum.concurrency = MAX_CONN
		return
	}

	sum.concurrency = c

}

//createOutputFile ...
func (sum *summon) createOutputFile(opath string) error {

	var fname string

	if opath != "" {
		fname = opath
	} else {
		currDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("Error while getting pwd : %v", err)
		}
		fname = currDir + "/" + sum.fileName
	}

	if _, err := os.Stat(fname); !os.IsNotExist(err) {
		return fmt.Errorf("File already exists : %v", fname)
	}

	out, err := os.OpenFile(fname, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0755)

	if err != nil {
		return fmt.Errorf("Error while creating file : %v", err)
	}

	sum.out = out

	return nil
}

//run is basically the start method
func (sum *summon) run() error {

	isSupported, contentLength, err := getRangeDetails(sum.uri)

	log.Printf("Multiple Connections Supported : %v | Got Content Length : %v", isSupported, contentLength)
	log.Printf("Using %v connections", sum.concurrency)

	if err != nil {
		return err
	}

	return sum.process(contentLength)

}

//process is the manager method
func (sum *summon) process(contentLength int) error {

	//Close the output file after everything is done
	defer sum.out.Close()

	split := contentLength / sum.concurrency

	wg := &sync.WaitGroup{}
	index := 0

	for i := 0; i < contentLength; i += split + 1 {
		j := i + split
		if j > contentLength {
			j = contentLength
		}

		f, err := os.CreateTemp("", sum.fileName+".*.part")
		if err != nil {
			return err
		}
		defer f.Close()
		defer os.Remove(f.Name())

		sum.chunks[index] = f

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

	var w int64
	//maps are not ordered hence using for loop
	for i := 0; i < len(sum.chunks); i++ {
		handle := sum.chunks[i]
		handle.Seek(0, 0) //We need to seek because read and write cursor are same and the cursor would be at the end.
		written, err := io.Copy(sum.out, handle)
		if err != nil {
			return err
		}
		w += written
	}

	log.Printf("Wrote to File : %v, Written bytes : %v", sum.out.Name(), w)

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

	//Get the handle
	sum.RLock()
	handle := sum.chunks[index]
	sum.RUnlock()

	_, sc, err := getDataAndWriteToFile(request, handle)
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
		Timeout: 5 * time.Second,
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

//getDataAndWriteToFile will get the response and write to file
func getDataAndWriteToFile(request *http.Request, f *os.File) (int64, int, error) {

	client := http.Client{
		Timeout: 0,
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, response.StatusCode, fmt.Errorf("Error while doing request : %v", err)
	}
	defer response.Body.Close()

	w, err := io.Copy(f, response.Body)
	if err != nil {
		return 0, response.StatusCode, fmt.Errorf("Error while copying response to file : %v", err)
	}

	return w, response.StatusCode, nil
}
