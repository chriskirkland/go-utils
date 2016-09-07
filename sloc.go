package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("example")
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

type fileLines struct {
	filename        string
	codeLines       int
	commentLines    int
	whitespaceLines int
}

func (this *fileLines) join(f fileLines) {
	this.codeLines += f.codeLines
	this.commentLines += f.commentLines
	this.whitespaceLines += f.whitespaceLines
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}

func getFileStats(filename string) fileLines {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	res := fileLines{filename: filename}

	// read file line by line
	inComment := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if err = scanner.Err(); err != nil {
			log.Fatal(err)
		}

		line = strings.TrimSpace(line)
		if len(line) == 0 {
			res.whitespaceLines++
		} else if strings.HasPrefix(line, "//") {
			res.commentLines++
		} else if strings.HasPrefix(line, "/*") {
			if !strings.HasSuffix(line, "*/") {
				inComment = true
			}
			res.commentLines++
		} else if inComment {
			if strings.HasSuffix(line, "*/") {
				inComment = false
			}
			res.commentLines++
		} else {
			res.codeLines++
		}
	}

	return res
}

func genFileProcessor(out chan<- fileLines) func(string, os.FileInfo, error) error {
	return func(path string, info os.FileInfo, err error) error {
		// ignore non-Golang files
		if !strings.HasSuffix(path, ".go") {
			log.Debug("ignoring", path)
			return nil
		}

		if err != nil {
			log.Error(err)
			return nil
		}

		log.Debug("fileProcessor", path)
		out <- getFileStats(path)
		return nil
	}
}

func processResults(results <-chan fileLines, done chan<- bool) {
	total := fileLines{filename: "TOTAL"}
	var data [][]string

	for res := range results {
		log.Infof("%+v\n", res)

		total.join(res)
		data = append(data, []string{
			res.filename,
			strconv.Itoa(res.whitespaceLines),
			strconv.Itoa(res.commentLines),
			strconv.Itoa(res.codeLines),
		})
	}

	// print table
	fmt.Println()
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"FILENAME", "White Space", "Comment", "Code"})
	table.SetFooter([]string{
		total.filename,
		strconv.Itoa(total.whitespaceLines),
		strconv.Itoa(total.commentLines),
		strconv.Itoa(total.codeLines),
	})
	table.SetBorder(false)
	table.AppendBulk(data)
	table.Render()

	done <- true
}

func main() {
	loggingLevels := map[string]logging.Level{
		"CRITICAL": logging.CRITICAL,
		"DEBUG":    logging.DEBUG,
		"ERROR":    logging.ERROR,
		"INFO":     logging.INFO,
		"NOTICE":   logging.NOTICE,
		"WARNING":  logging.WARNING,
	}

	// parse flags
	loggingFlag := flag.String("loglevel", "INFO", "log level")
	flag.Parse()
	files := flag.Args()
	loggingLevel, ok := loggingLevels[*loggingFlag]
	if !ok {
		log.Fatalf("Invalid log level: found %v", loggingLevel)
	}
	fmt.Printf("loggingLevel %v\n", loggingLevel)

	// setup logging
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	formatter := logging.NewBackendFormatter(backend, format)
	leveledBackend := logging.AddModuleLevel(backend)
	leveledBackend.SetLevel(loggingLevel, "")
	logging.SetBackend(leveledBackend, formatter)

	log.Notice("notice")
	log.Warning("warning")
	log.Error("err")
	log.Critical("crit")

	results := make(chan fileLines)
	done := make(chan bool)

	// start results goroutine
	go processResults(results, done)

	// walk files
	fileProcessor := genFileProcessor(results)
	for _, file := range files {
		log.Debug("processing", file)
		err := filepath.Walk(file, fileProcessor)
		if err != nil {
			log.Fatal(err)
		}
	}
	close(results)

	// wait for results to be processed
	<-done
}
