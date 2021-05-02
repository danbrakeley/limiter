package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/danbrakeley/commandline"
	"github.com/danbrakeley/frog"
)

var Version = "v0.3.alpha"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "limiter %s\n"+
			"\n"+
			"Usage: %s <max-concurrency>\n"+
			"\n"+
			"Description:\n"+
			"  Execute tasks concurrently until <max-concurrency> tasks are running.\n"+
			"  If there are additional tasks, run them as soon as earlier tasks complete.\n"+
			"\n"+
			"  Tasks are passed, one to a line, via stdin.\n"+
			"",
			Version, filepath.Base(os.Args[0]),
		)
		os.Exit(1)
	}

	var limit int64
	var err error

	if limit, err = strconv.ParseInt(os.Args[1], 10, 32); err != nil {
		fmt.Fprintf(os.Stderr, "unable to parse first argument: %v\n", err)
		os.Exit(2)
	}
	if limit <= 0 {
		fmt.Fprintf(os.Stderr, "max-concurrency must be an integer greater than zero, given %d\n", limit)
		os.Exit(3)
	}

	log := frog.New(frog.Auto)

	if err := readAndRunTasks(log, int(limit), os.Stdin); err != nil {
		log.Error("Aborted", frog.Err(err))
	}

	log.Close()
}

type Task struct {
	TaskID  int64
	LineNum int64
	Cmd     string
}

func readAndRunTasks(log frog.Logger, limit int, taskReader io.Reader) error {
	log.Info("Starting", frog.Int("limit", limit))

	var wg sync.WaitGroup
	chTasks := make(chan Task)

	// start task runners
	wg.Add(limit)
	for i := 0; i < limit; i++ {
		thread := i
		go func() {
			// var l frog.Logger
			for t := range chTasks {
				fthread := frog.Int("thread", thread)
				fline := frog.Int64("line", t.LineNum)
				fcmd := frog.String("cmd", t.Cmd)

				// if l == nil {
				// 	l = frog.AddAnchor(log)
				// }
				// l.Transient("Running", fthread, fline, fcmd)

				args, err := commandline.Parse(t.Cmd)
				if err != nil {
					log.Error("failed to parse task as commandline", fthread, fline, fcmd, frog.Err(err))
					continue
				}
				cmd := exec.Command(args[0], args[1:]...)
				// TODO: this messes up frog's cursor movements, so need to build something to pipe
				// anything sent to Stderr to actual frog log lines
				//cmd.Stderr = os.Stderr
				err = cmd.Run()
				if err != nil {
					log.Error("command failed", fthread, fline, fcmd, frog.Err(err))
					continue
				}

				log.Info("Completed", fthread, fline, fcmd)
			}
			// frog.RemoveAnchor(l)
			wg.Done()
		}()
	}

	// feed data to runners
	scanner := bufio.NewScanner(taskReader)
	var taskID int64
	var lineNum int64
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++
		if len(line) > 0 && line[0] != '#' {
			chTasks <- Task{TaskID: taskID, LineNum: lineNum, Cmd: line}
			taskID++
		}
	}
	if err := scanner.Err(); err != nil {
		log.Error("Error reading stdio", frog.Err(err))
	}

	// signal task runers to shutdown after their current task completes
	close(chTasks)
	// wait for running tasks to complete
	wg.Wait()

	log.Info("Done")

	return nil
}
