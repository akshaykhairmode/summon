package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type resume struct {
	downloaded   int64
	start        int64
	end          int64
	tempFilePath string
}

type meta struct {
	ChunkPaths map[int64]string  `json:"chunkPaths"` //Key is index & value is absolute path of chunk
	Range      map[int64][]int64 `json:"range"`      //Key is index & value is the initial range which was used. 0 being start and 1 being the end
}

//getMetaData will set the meta data to summon
func (sum *summon) setMetaData(fpath string) error {

	fname := sum.getMetaFileName()
	meta := meta{}

	data, err := os.ReadFile(fname)
	if err != nil {
		return err
	}

	decoded, err := decode(data)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(decoded, &meta); err != nil {
		return err
	}

	sum.metaData = meta

	return nil
}

func (sum summon) getMetaFileName() string {
	return sum.fileDetails.fileDir + sum.separator + "." + sum.fileDetails.fileName + ".summon.meta"
}

//canBeResumed tells us if the file can be resumed
func (sum *summon) canBeResumed(fpath string) (bool, []string) {

	//If meta file does not exist we cant resume the download
	if !fileExists(sum.getMetaFileName()) {
		return false, []string{}
	}

	//Set the metadata
	if err := sum.setMetaData(fpath); err != nil {
		return false, []string{}
	}

	parts := []string{}

	for index, filePath := range sum.metaData.ChunkPaths {

		start, end := sum.metaData.Range[index][0], sum.metaData.Range[index][1]

		finfo, err := os.Stat(filePath)
		if err != nil {
			LogWriter.Println(err)
			return false, parts
		}

		//This size is in bytes
		contentL := finfo.Size()

		sum.fileDetails.chunks[index] = nil
		sum.fileDetails.resume[index] = resume{downloaded: int64(contentL), end: end, start: start, tempFilePath: filePath}
	}

	return true, parts
}

func (sum *summon) resumeDownload(wg *sync.WaitGroup) error {

	for index := range sum.fileDetails.chunks {

		var start, end, total int64

		//The previous start range + the bytes we have downloaded will give us the new range
		start = sum.fileDetails.resume[index].start + sum.fileDetails.resume[index].downloaded

		//We will use the same end here as we are going to use same concurrency
		end = sum.fileDetails.resume[index].end

		//We need to use the old total value only
		total = sum.fileDetails.resume[index].end - sum.fileDetails.resume[index].start

		//We will start the progress from last time, so we set the current progress directly
		sum.progressBar.p[index] = &progress{curr: sum.fileDetails.resume[index].downloaded, total: total}

		f, err := os.OpenFile(sum.fileDetails.resume[index].tempFilePath, os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return err
		}

		//Set the file handles so that combine can use them
		sum.fileDetails.chunks[index] = f

		//If chunk is already completed skip download
		if start > sum.fileDetails.resume[index].end {
			continue
		}

		contentRange := fmt.Sprintf("%d-%d", start, end)

		wg.Add(1)
		go sum.downloadFileForRange(wg, contentRange, index, f)
	}

	return nil
}

func (sum *summon) download(wg *sync.WaitGroup) error {

	index := int64(0)
	split := sum.fileDetails.contentLength / sum.concurrency
	meta := meta{ChunkPaths: make(map[int64]string), Range: make(map[int64][]int64)}

	for start := int64(0); start < sum.fileDetails.contentLength; start += split + 1 {
		end := start + split
		if end > sum.fileDetails.contentLength {
			end = sum.fileDetails.contentLength
		}

		//get temp file name
		partFileName, err := sum.getTempFileName(index, start, end)
		if err != nil {
			return err
		}

		//Create temp file
		f, err := os.Create(partFileName)
		if err != nil {
			return err
		}

		//Set metadata
		meta.ChunkPaths[index] = f.Name()
		meta.Range[index] = []int64{start, end}

		//init progressbar
		sum.progressBar.p[index] = &progress{curr: 0, total: end - start}

		//init temp files
		sum.fileDetails.chunks[index] = f

		contentRange := fmt.Sprintf("%d-%d", start, end)

		wg.Add(1)
		go sum.downloadFileForRange(wg, contentRange, index, f)
		index++
	}

	if !sum.isRangeSupported {
		return nil
	}

	sum.addMetadataToFile(meta)

	return nil

}

func (sum summon) addMetadataToFile(m meta) {
	//Add metadata to file
	metaFname := sum.getMetaFileName()
	metaData, err := json.Marshal(m)
	if err != nil {
		LogWriter.Printf("Error occured while marshalling json : %v", err)
	}

	finalData := bytes.NewBuffer(nil)
	if err := encode(metaData, finalData); err != nil {
		LogWriter.Printf("Error occured while encoding meta data : %v", err)
	}

	if err := os.WriteFile(metaFname, finalData.Bytes(), 0644); err != nil {
		LogWriter.Printf("Error occured while writing meta data : %v", err)
	}
}

//deleteFiles deletes the list of files provided
func (sum *summon) deleteFiles(chunks map[int64]*os.File, tempFileName ...string) error {

	for _, handle := range chunks {
		if handle == nil {
			continue
		}
		LogWriter.Printf("Removing file : %v, Err : %v", handle.Name(), os.Remove(handle.Name()))
	}

	for _, temp := range tempFileName {

		if !fileExists(temp) {
			continue
		}

		LogWriter.Printf("Removing file : %v, Err : %v", temp, os.Remove(temp))

	}

	return nil
}

//createTempOutputFile will create the final output file
func (sum *summon) createTempOutputFile() error {

	//Check if file already exists with same name
	if fileExists(sum.fileDetails.absolutePath) {
		return fmt.Errorf("File : %v already exists", sum.fileDetails.absolutePath)
	}

	tempOutFileName := sum.fileDetails.fileDir + sum.separator + "." + sum.fileDetails.fileName

	if isValid, parts := sum.canBeResumed(tempOutFileName); isValid {
		var shouldResume string
		fmt.Print("Looks like previous download was incomplete for this file, do you want to resume ? [Y/n] ")
		_, err := fmt.Scanln(&shouldResume)
		if err != nil {
			return err
		}

		if shouldResume == "Y" {
			sum.isResume = true
			sum.concurrency = int64(len(sum.fileDetails.chunks))
		} else {
			//Delete Temp file and chunks both
			if err := sum.deleteFiles(map[int64]*os.File{}, append(parts, tempOutFileName, sum.getMetaFileName())...); err != nil {
				return err
			}
		}
	}

	out, err := os.OpenFile(tempOutFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("Error while creating file : %v", err)
	}

	sum.fileDetails.tempOutFile = out

	return nil
}
