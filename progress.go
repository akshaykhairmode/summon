package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type progress struct {
	curr  int //curr is the current read till now
	total int //total bytes which we are supposed to read
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
