package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//canBeResumed tells us if the file can be resumed
func (sum *summon) canBeResumed(fpath string) (bool, map[int]*os.File) {

	dir := filepath.Dir(fpath)
	chunks := map[int]*os.File{}

	parts, _ := filepath.Glob(dir + sum.separator + "*summonp*")

	for _, absPath := range parts {

		spl := strings.Split(absPath, "_")

		index, _ := strconv.Atoi(spl[len(spl)-1])

		handle, err := os.OpenFile(absPath, os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			log.Println(err)
			return false, chunks
		}

		finfo, err := handle.Stat()
		if err != nil {
			log.Println(err)
			return false, chunks
		}

		contentL := finfo.Size()

		chunks[index] = handle
		sum.resume[index] = contentL
	}

	return true, chunks
}

func (sum *summon) cleanOldChunks(chunks map[int]*os.File, tempFileName string) error {

	for _, handle := range chunks {
		handle.Close()
		if err := os.Remove(handle.Name()); err != nil {
			return err
		}
	}

	return os.Remove(tempFileName)
}

//createTempOutputFile ...
func (sum *summon) createTempOutputFile() error {

	//Check if file already exists with same name
	if fileExists(sum.absolutePath) {
		return fmt.Errorf("File already exists")
	}

	tempOutFileName := filepath.Dir(sum.absolutePath) + sum.separator + "." + sum.fileName

	if fileExists(tempOutFileName) {
		if isValid, chunks := sum.canBeResumed(tempOutFileName); isValid {
			var shouldResume string
			fmt.Print("Looks like previous download was incomplete for this file, do you want to resume ? [Y/n]")
			_, err := fmt.Scanln(&shouldResume)
			if err != nil {
				return err
			}

			if shouldResume == "Y" {
				sum.chunks = chunks
				sum.concurrency = len(chunks)
			} else {
				if err := sum.cleanOldChunks(chunks, tempOutFileName); err != nil {
					return err
				}
			}
		}
	}

	out, err := os.OpenFile(tempOutFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("Error while creating file : %v", err)
	}

	sum.tempOut = out

	return nil
}
