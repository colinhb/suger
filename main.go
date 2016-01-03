package main

import (
	"regexp"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	suger "github.com/colinhb/suger/libsuger"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"
	"strings"
)

func signalHandler(ch chan os.Signal) {
	for sig := range ch {
		log.Println("Caught signal:", sig)
		os.Exit(0)
	}
}

// Heredoc is a helper to do multi-line strings
func Heredoc(doc string) string {
	// split into lines
	lines := strings.Split(doc, "\n")

	// trim first and last lines
	if len(lines) < 3 {
		return ""
	}
	lines = lines[1:len(lines)-1]

	// reduce for min tab indent
	var min string // accumulator
	re := regexp.MustCompile(`^(\t*)\S+`) // submatch is 0+ tabs at start of line
	for i := 0; i < len(lines); i++ {
		tabs := re.FindStringSubmatch(lines[0])[1] // e.g. "\t\t\t"
		if i == 0 { // init accumulator 
			min = tabs
			continue
		} 
		if len(tabs) < len(min) { // len() is ok - all runes are "\t"
			min = tabs
		}
	}

	// map in-place for unindent
	for i, s := range lines {
		lines[i] = strings.Replace(s, min, "", 1) // 1 replaces only left most occurance
	} 

	// join lines and return
	return strings.Join(lines, "\n")
}

func main() {
	// handle ^C
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go signalHandler(ch)

	usage := Heredoc(`
		Usage of suger:
			suger crawl [flags]
				crawl classification database
			suger scrape [flags]
				scrape downloaded html files
		(Use the -h flag for help with each subcommand.)
	`)

	// if no subcommand, give usage information
	if len(os.Args) == 1 {
		fmt.Println(usage)
		return
	}

	// common flag vars
	var htmlDir string

	// crawl flag vars
	var start int
	var count int
	var workers int

	// scrape flag vars
	var out string

	// crawl flagset 
	crawlFlags := flag.NewFlagSet("crawl", flag.ExitOnError)
	crawlFlags.IntVar(&start, "start", 1, "start at this result")
	crawlFlags.IntVar(&count, "count", 25, "crawl this many results")
	crawlFlags.StringVar(&htmlDir, "html", "html", "directory to write HTML files")
	crawlFlags.IntVar(&workers, "workers", 1, "number of workers")

	// scrape flagset
	scrapeFlags := flag.NewFlagSet("scrape", flag.ExitOnError)
	scrapeFlags.StringVar(&htmlDir, "html", "html", "directory to read HTML files")
	scrapeFlags.StringVar(&out, "out", "out", "directory for output")

	// switch on subcommand
	switch os.Args[1] {
		case "crawl":
			crawlFlags.Parse(os.Args[2:])
			crawlCmd(start, count, htmlDir, workers)
		case "scrape":
			scrapeFlags.Parse(os.Args[2:])
			scrapeCmd(htmlDir, out)
		default:
			fmt.Printf("Error: %q is not valid subcommand.\n", os.Args[1])
			fmt.Println(usage)
	}
}


// crawlCmd() is called by the switch in main()
func crawlCmd(start int, count int, htmlDir string, workers int) {
	// make channels
	jobs := make(chan suger.Job, workers)
	results := make(chan suger.Result, workers)
	done := make(chan bool, workers)

	
	j, err := suger.NewJob(start, count)
	if err != nil {
		log.Fatal(err)
	}
	parts, err := j.Partition(workers)
	log.Println("Parts:", parts)
	for i := 0; i < len(parts); i++ {
		jobs <- parts[i]	
	}

	remaining := len(parts)

	for {
		select {
		case j := <-jobs:
			log.Println("Received Job:", j)
			if j.Error != nil {
				log.Println(j.Error)
				log.Printf("Sleeping for 30 seconds because of error.\n")
				time.Sleep(time.Second * 30)
			}
			if j.IsDone() {
				done <- true
			} else {
				c, _ := suger.NewCrawler()
				go c.Crawl(j, results, jobs)
			}
		case r := <-results:
			file := fmt.Sprintf("%v/title-%v-%v.html", htmlDir, r.Page, r.Row)
			err = ioutil.WriteFile(file, r.HTML, 0644)
			if err != nil {
				log.Fatal(err)
			}
		case <-done:
			remaining = remaining - 1
			log.Printf("One worker finished;  %v workers remaining.", remaining)
			if remaining == 0 {
				os.Exit(0)
			}
		}
	}
}

func scrapeCmd(htmlDir string, out string) {
	var titles []*suger.Title
	files, err := ioutil.ReadDir(htmlDir)
	for _, fileInfo := range files {
		path := fmt.Sprintf("%s/%s", htmlDir, fileInfo.Name())
		html, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		title, err := suger.NewTitleFromHTML(html)
		if err != nil {
			log.Fatal(err)
		}
		titles = append(titles, title)
	}

	//
	// JSON
	//

	fileName := fmt.Sprintf("%s/%s", out, "out.json")
	f, err := os.Create(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	json, err := json.MarshalIndent(titles, "", "	")
	_, err = w.Write(json)
	if err != nil {
		log.Fatal(err)
	}
	err = w.Flush()
	if err != nil {
		log.Fatal(err)
	}

}
