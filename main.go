package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	MAX_CONN      = 20
	MIN_CONN      = 1
	PROGRESS_SIZE = 30
)

type summon struct {
	concurrency   int               //No. of connections
	uri           string            //URL of the file we want to download
	chunks        map[int]*os.File  //Map of temporary files we are creating
	err           error             //used when error occurs inside a goroutine
	startTime     time.Time         //to track time took
	fileName      string            //name of the file we are downloading
	out           *os.File          //output / downloaded file
	progressBar   map[int]*progress //index => progress
	stop          chan error        //to handle stop signals from terminal
	*sync.RWMutex                   //mutex to lock the maps which accessing it concurrently
}

type progress struct {
	curr  int //curr is the current read till now
	total int //total bytes which we are supposed to read
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

	//get the user kill signals
	go sum.catchSignals()

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
	sum.progressBar = make(map[int]*progress)
	sum.stop = make(chan error)

	if err := sum.createOutputFile(*o); err != nil {
		return nil, err
	}

	return sum, nil

}

//setConcurrency set the concurrency as per min and max
func (sum *summon) setConcurrency(c int) {

	if c <= MIN_CONN {
		sum.concurrency = MIN_CONN
		return
	}

	if c >= MAX_CONN {
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
		sum.progressBar[index] = &progress{curr: 0, total: j - i}

		wg.Add(1)
		go sum.downloadFileForRange(wg, sum.uri, strconv.Itoa(i)+"-"+strconv.Itoa(j), index, f)
		index++
	}

	stop := make(chan struct{})

	//Keep Printing Progress
	go sum.startProgressBar(stop)
	wg.Wait()

	stop <- struct{}{}

	if sum.err != nil {
		os.Remove(sum.out.Name())
		return sum.err
	}

	return sum.combineChunks()
}

func (sum *summon) startProgressBar(stop chan struct{}) {

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for i := 0; i < len(sum.progressBar); i++ {

				sum.RLock()
				p := *sum.progressBar[i]
				sum.RUnlock()

				printProgress(i, p)
			}

			//Move cursor back
			for i := 0; i < len(sum.progressBar); i++ {
				fmt.Print("\033[F")
			}

		case <-stop:
			for i := 0; i < len(sum.progressBar); i++ {
				sum.RLock()
				p := *sum.progressBar[i]
				sum.RUnlock()
				printProgress(i, p)
			}
			return
		}
	}

}

func printProgress(index int, p progress) {

	s := strings.Builder{}

	percent := math.Round((float64(p.curr) / float64(p.total)) * 100)

	n := int((percent / 100) * PROGRESS_SIZE)

	s.WriteString("[")
	for i := 0; i < PROGRESS_SIZE; i++ {
		if i <= n {
			s.WriteString(">")
		} else {
			s.WriteString(" ")
		}
	}
	s.WriteString("]")
	s.WriteString(fmt.Sprintf(" %v%%", percent))

	fmt.Printf("Connection %d  - %s\n", index+1, s.String())
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
func (sum *summon) downloadFileForRange(wg *sync.WaitGroup, u, r string, index int, handle io.Writer) {

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
func (sum *summon) getDataAndWriteToFile(request *http.Request, f io.Writer, index int) (int, error) {

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
	var readTotal int

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

func (sum *summon) readBody(response *http.Response, f io.Writer, buf []byte, readTotal *int, index int) error {

	r, err := response.Body.Read(buf)

	if r > 0 {
		f.Write(buf[:r])
	}

	if err != nil {
		return err
	}

	*readTotal += r

	sum.Lock()
	sum.progressBar[index].curr = *readTotal
	sum.Unlock()

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
		for i := 0; i < len(sum.chunks); i++ {
			sum.stop <- fmt.Errorf("got stop signal : %v", s)
		}
	}()
}
