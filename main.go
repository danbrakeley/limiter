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

var Version = "v0.2.0"

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

func readAndRunTasks(log frog.Logger, limit int, taskReader io.Reader) error {
	log.Info("Starting", frog.Int("limit", limit))

	var wg sync.WaitGroup
	chTasks := make(chan string)

	// start task runners
	wg.Add(limit)
	for i := 0; i < limit; i++ {
		id := i
		go func() {
			var l frog.Logger
			for t := range chTasks {
				if l == nil {
					l = frog.AddFixedLine(log)
				}
				l.Transient("Running", frog.String("cmd", t), frog.Int("thread", id))

				args, err := commandline.Parse(t)
				if err != nil {
					l.Error("failed to parse task as commandline", frog.Err(err), frog.String("cmd", t))
					continue
				}
				cmd := exec.Command(args[0], args[1:]...)
				cmd.Stderr = os.Stderr
				err = cmd.Run()
				if err != nil {
					l.Error("command failed", frog.Err(err), frog.String("cmd", t))
					continue
				}

				l.Info("Completed", frog.String("cmd", t), frog.Int("thread", id))
			}
			frog.RemoveFixedLine(l)
			wg.Done()
		}()
	}

	// feed data to runners
	scanner := bufio.NewScanner(taskReader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 && line[0] != '#' {
			chTasks <- line
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
