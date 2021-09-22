package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type summon struct {
	concurrency   uint32      //No. of connections
	uri           string      //URL of the file we want to download
	isResume      bool        //is this a resume request
	err           error       //used when error occurs inside a goroutine
	startTime     time.Time   //to track time took
	fileDetails   fileDetails //will hold the file related details
	progressBar   progressBar //index => progress
	stop          chan error  //to handle stop signals from terminal
	separator     string      //store the path separator based on the OS
	*sync.RWMutex             //mutex to lock the maps which accessing it concurrently
}

type fileDetails struct {
	chunks        map[uint32]*os.File //Map of part files we are creating
	fileName      string              //name of the file we are downloading
	fileDir       string              //dir of the file
	absolutePath  string              //absolute path of the output file
	tempOutFile   *os.File            //output / downloaded file
	resume        map[uint32]resume   //how much is downloaded
	contentLength uint32
}

func NewSummon() (*summon, error) {

	args := arguments{}

	parseFlags(&args)

	if args.help {
		flag.PrintDefaults()
		fmt.Println("\nExample Usage - $GOBIN/summon -c 5 http://www.africau.edu/images/default/sample.pdf")
		return nil, nil
	}

	if args.verbose {
		LogWriter = DevLogger{}
	} else {
		LogWriter = ProdLogger{}
	}

	sum := new(summon)

	fileURL, err := validate()
	if err != nil {
		return sum, err
	}

	sum.uri = fileURL
	sum.fileDetails.chunks = make(map[uint32]*os.File)
	sum.startTime = time.Now()
	sum.fileDetails.fileName = filepath.Base(sum.uri)
	sum.RWMutex = &sync.RWMutex{}
	sum.progressBar.RWMutex = &sync.RWMutex{}
	sum.progressBar.p = make(map[uint32]*progress)
	sum.stop = make(chan error)
	sum.separator = string(os.PathSeparator)
	sum.fileDetails.resume = make(map[uint32]resume)

	sum.setConcurrency(args.connections)
	sum.setAbsolutePath(args.outputFile)
	sum.setFileDir()

	if err := sum.createTempOutputFile(); err != nil {
		return nil, err
	}

	return sum, nil

}

func validate() (string, error) {
	if len(flag.Args()) <= 0 {
		return "", fmt.Errorf("Please pass file url")
	}

	u := flag.Args()[0]
	uri, err := url.ParseRequestURI(u)
	if err != nil {
		return "", fmt.Errorf("Passed URL is invalid")
	}

	return uri.String(), nil
}

//process is the manager method
func (sum *summon) process() error {

	wg := &sync.WaitGroup{}

	if sum.isResume {
		sum.resumeDownload(wg)
	} else {
		if err := sum.download(wg); err != nil {
			return err
		}
	}

	stop := make(chan struct{})

	//Keep Printing Progress
	go sum.startProgressBar(stop)
	wg.Wait()

	stop <- struct{}{}

	if sum.err != nil {
		return sum.err
	}

	return sum.combineChunks()
}

func (sum summon) getTempFileName(index, start, end uint32) (string, error) {

	meta := fmt.Sprintf("%d#%d#%d", index, start, end)

	encoded := bytes.NewBuffer(nil)
	if err := encode(meta, encoded); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%s.%s.sump_%s", sum.fileDetails.fileDir, sum.separator, sum.fileDetails.fileName, encoded.String()), nil

}

//setConcurrency set the concurrency as per min and max
func (sum *summon) setConcurrency(c uint32) {

	//We use default connections in case no concurrency is passed
	if c <= 0 {
		log.Println("Using default number of connections", DEFAULT_CONN)
		sum.concurrency = DEFAULT_CONN
		return
	}

	if c >= MAX_CONN {
		sum.concurrency = MAX_CONN
		return
	}

	sum.concurrency = c
}

func (sum *summon) setAbsolutePath(opath string) error {

	if opath == "" {
		opath = filepath.Base(sum.uri)
	}

	if filepath.IsAbs(opath) {
		sum.fileDetails.absolutePath = opath
		return nil
	}

	absPath, err := filepath.Abs(opath)
	if err != nil {
		LogWriter.Printf("error while getting absolute path : %v", err)
		return err
	}

	sum.fileDetails.absolutePath = absPath

	return nil
}

func (sum *summon) setFileDir() {
	sum.fileDetails.fileDir = filepath.Dir(sum.fileDetails.absolutePath)
}

//combineChunks will combine the chunks in ordered fashion starting from 1
func (sum *summon) combineChunks() error {

	var w int64
	//maps are not ordered hence using for loop
	for i := uint32(0); i < uint32(len(sum.fileDetails.chunks)); i++ {
		handle := sum.fileDetails.chunks[i]
		handle.Seek(0, 0) //We need to seek because read and write cursor are same and the cursor would be at the end.
		written, err := io.Copy(sum.fileDetails.tempOutFile, handle)
		if err != nil {
			return err
		}
		w += written
	}

	tempFileName := sum.fileDetails.tempOutFile.Name()

	log.Printf("Wrote to Temp File : %v, Written bytes : %v", tempFileName, w)
	sum.fileDetails.tempOutFile.Close()

	finalFileName := filepath.Dir(sum.fileDetails.absolutePath) + sum.separator + sum.fileDetails.fileName

	if err := os.Rename(tempFileName, finalFileName); err != nil {
		return fmt.Errorf("error occured while renaming file : %v", err)
	}

	return nil
}

//downloadFileForRange will download the file for the provided range and set the bytes to the chunk map, will set summor.error field if error occurs
func (sum *summon) downloadFileForRange(wg *sync.WaitGroup, u, r string, index uint32, handle io.Writer) {

	defer wg.Done()

	request, err := http.NewRequest("GET", u, strings.NewReader(""))
	if err != nil {
		sum.err = err
		return
	}

	request.Header.Add("Range", "bytes="+r)

	sc, err := sum.getDataAndWriteToFile(request, handle, index)
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
func getRangeDetails(u string) (bool, uint32, error) {

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

	cl, err := parseUint32(conLen)
	if err != nil {
		return false, 0, fmt.Errorf("Error Parsing content length : %v", err)
	}

	//Accept-Ranges: bytes
	if headers.Get("Accept-Ranges") == "bytes" {
		return true, cl[0], nil
	}

	return false, cl[0], nil

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
func (sum *summon) getDataAndWriteToFile(request *http.Request, f io.Writer, index uint32) (int, error) {

	client := http.Client{
		Timeout: 0,
	}

	response, err := client.Do(request)
	if err != nil {
		return response.StatusCode, fmt.Errorf("Error while doing request : %v", err)
	}
	defer response.Body.Close()

	//we make buffer of 500 bytes and try to read 500 bytes every iteration.
	var buf = make([]byte, 500)
	var readTotal uint32

	for {
		select {
		case cErr := <-sum.stop:
			return response.StatusCode, cErr
		default:
			err := sum.readBody(response, f, buf, &readTotal, index)
			if err == io.EOF {
				return response.StatusCode, nil
			}

			if err != nil {
				return response.StatusCode, err
			}
		}
	}
}

func (sum *summon) readBody(response *http.Response, f io.Writer, buf []byte, readTotal *uint32, index uint32) error {

	r, err := response.Body.Read(buf)

	if r > 0 {
		f.Write(buf[:r])
	}

	if err != nil {
		return err
	}

	*readTotal += uint32(r)

	sum.progressBar.Lock()
	sum.progressBar.p[index].curr = *readTotal
	sum.progressBar.Unlock()

	return nil
}