package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type resume struct {
	downloaded   uint32
	start        uint32
	end          uint32
	tempFilePath string
}

//canBeResumed tells us if the file can be resumed
func (sum *summon) canBeResumed(fpath string) (bool, []string) {

	dir := filepath.Dir(fpath)

	parts, _ := filepath.Glob(dir + sum.separator + "." + sum.fileDetails.fileName + "*sump*")

	for _, absPath := range parts {

		spl := strings.Split(absPath, "_")

		encData := spl[len(spl)-1]

		decData, err := decode(encData)
		if err != nil {
			LogWriter.Println(err)
			return false, parts
		}

		resumeVals := strings.Split(decData, "#")

		metaData, err := parseUint32(resumeVals[0], resumeVals[1], resumeVals[2])
		if err != nil {
			LogWriter.Println(err)
			return false, parts
		}

		index, start, end := metaData[0], metaData[1], metaData[2]

		LogWriter.Printf("Read Meta Data for index : %d, start : %d, end : %d", index, start, end)

		finfo, err := os.Stat(absPath)
		if err != nil {
			LogWriter.Println(err)
			return false, parts
		}

		contentL := finfo.Size()

		sum.fileDetails.chunks[index] = nil
		sum.fileDetails.resume[index] = resume{downloaded: uint32(contentL), end: end, start: start, tempFilePath: absPath}
	}

	return true, parts
}

func (sum *summon) resumeDownload(wg *sync.WaitGroup) error {

	for index := range sum.fileDetails.chunks {

		//The previous start range + the bytes we have downloaded will + 1 will give us the new range
		start := sum.fileDetails.resume[index].start + sum.fileDetails.resume[index].downloaded + 1
		//We will use the same end here as we are going to use same concurrency
		end := sum.fileDetails.resume[index].end
		//We need to use the old total value only
		total := sum.fileDetails.resume[index].end - sum.fileDetails.resume[index].start
		//We will start the progress from last time, so we set the current progress directly
		sum.progressBar.p[index] = &progress{curr: sum.fileDetails.resume[index].downloaded, total: total}

		LogWriter.Printf("Resume Details - Start : %d , End : %d, Total : %d for index : %v", start, end, total, index)

		contentRange := fmt.Sprintf("%d-%d", start, end)

		f, err := os.OpenFile(sum.fileDetails.resume[index].tempFilePath, os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return err
		}

		wg.Add(1)
		go sum.downloadFileForRange(wg, contentRange, index, f)
	}

	return nil
}

func (sum *summon) download(wg *sync.WaitGroup) error {

	index := uint32(0)
	split := sum.fileDetails.contentLength / sum.concurrency
	for start := uint32(0); start < sum.fileDetails.contentLength; start += split + 1 {
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

		//init progressbar
		sum.progressBar.p[index] = &progress{curr: 0, total: end - start}

		//init temp files
		sum.fileDetails.chunks[index] = f

		contentRange := fmt.Sprintf("%d-%d", start, end)

		wg.Add(1)
		go sum.downloadFileForRange(wg, contentRange, index, f)
		index++
	}

	return nil

}

func (sum *summon) cleanUp(chunks map[uint32]*os.File, tempFileName ...string) error {

	for _, handle := range chunks {

		if handle == nil {
			continue
		}

		handle.Close()

		if err := os.Remove(handle.Name()); err != nil {
			return err
		}
	}

	for _, temp := range tempFileName {

		LogWriter.Printf("Removing file : %v", temp)

		if !fileExists(temp) {
			continue
		}

		os.Remove(temp)

	}

	return nil
}

//createTempOutputFile ...
func (sum *summon) createTempOutputFile() error {

	//Check if file already exists with same name
	if fileExists(sum.fileDetails.absolutePath) {
		return fmt.Errorf("File :%v already exists", sum.fileDetails.absolutePath)
	}

	tempOutFileName := sum.fileDetails.fileDir + sum.separator + "." + sum.fileDetails.fileName

	if fileExists(tempOutFileName) {
		if isValid, parts := sum.canBeResumed(tempOutFileName); isValid {
			var shouldResume string
			fmt.Print("Looks like previous download was incomplete for this file, do you want to resume ? [Y/n]")
			_, err := fmt.Scanln(&shouldResume)
			if err != nil {
				return err
			}

			if shouldResume == "Y" {
				sum.isResume = true
				sum.concurrency = uint32(len(sum.fileDetails.chunks))
			} else {
				if err := sum.cleanUp(sum.fileDetails.chunks, append(parts, tempOutFileName)...); err != nil {
					return err
				}
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
