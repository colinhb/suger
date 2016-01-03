// Libsuger is a micro-library for the executable suger. Suger is a tool to crawl and scrape the film classification database of Singapore's Media Development Authority (MDA).
package libsuger

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	// "log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

// Job is a type that stores certain state information used by the Crawl method the Crawler type. Its only exported field is Error, which contains the last error recorded by Crawl method.
type Job struct {
	start int
	stop  int
	Error error
}

// NewJob creates a Job from the first result you want to crawl (start) and the number of results (count) that you want to crawl. It returns an error if start or count are less than one.
func NewJob(start int, count int) (Job, error) {
	j := Job{}
	if !(start > 0 && count > 0) {
		s := "start (%v) and count (%v) must be greater than zero."
		err := errors.New(fmt.Sprintf(s, start, count))
		return j, err
	}
	j.start = start
	j.stop = start + count
	return j, nil
}

func (j Job) next() Job {
	j.start = j.start + 1
	return j
}

// IsDone returns true if there are no more results to crawl (i.e., all have been successfully crawled.)
func (j Job) IsDone() bool {
	return j.start >= j.stop
}

// Partition returns a slice of non-overlapping Jobs of roughly equal count.
func (j Job) Partition(n int) ([]Job, error) {
	var sl []Job
	count := j.stop - j.start
	if count < n {
		s := "the number of partitions (%v) must be greater than count (%v)."
		err := errors.New(fmt.Sprintf(s, n, count))
		return sl, err
	}
	q := count / n
	if q == 0 {
		q = 1
	}
	for i := 0; i < n; i++ {
		start := j.start + (q * i)
		part, _ := NewJob(start, q)
		sl = append(sl, part)
	}
	last := sl[n-1]
	last.stop = j.stop
	return sl, nil
}

func (j Job) page() int {
	p := ((j.start - 1) / 20) + 1
	return int(p)
}

func (j Job) row() int {
	r := ((j.start - 1) % 20)
	return int(r)
}

// Rating is a simple type to hold a single rating (e.g. "No Children Under 16") and decision (e.g. "Passed Clean").
type Rating struct {
	Rating   string
	Decision string
}

// Title is a simple type to hold the Name, URL, and various Ratings for a title in the database.
type Title struct {
	Name    string
	Ratings []Rating
	URL     string
}

func NewTitleFromHTML(html []byte) (*Title, error) {
	reader := bytes.NewReader(html)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(doc.Find("#lblTitle").Text())
	var ratings []Rating
	ratImg := doc.Find("div#content td img")
	// use something other than Each....
	ratImg.Each(func(i int, s *goquery.Selection) {
		rat, ok := s.Attr("alt")
		if !ok {
			err = errors.New("No 'alt' attribute.")
			return
		}
		dec := s.Closest("td").Next().Text()
		rating := Rating{
			Rating:   rat,
			Decision: dec,
		}
		ratings = append(ratings, rating)
	})
	if err != nil {
		return nil, err
	}
	u, ok := doc.Find("#form1").Attr("action")
	if !ok {
		err = errors.New("No 'action' attribute.")
		return nil, err
	}
	u = fmt.Sprintf("https://app.mda.gov.sg/Classification/Search/Film/%v", u)
	title := &Title{
		Name:    name,
		Ratings: ratings,
		URL:     u,
	}
	return title, nil
}

var orderedRatings []string = []string{
	"Restricted 21",
	"Matured Above 18",
	"No Children Under 16",
	"Parental Guidance 13",
	"Parental Guidance",
	"General Viewing",
}

// MaxRating returns the "highest" rating a title has been given. It's bool return value is false if the Title has no ratings (an ok pattern).
func (t *Title) MaxRating() (string, bool) {
	unique := make(map[string]struct{})
	for i := 0; i < len(t.Ratings); i++ {
		s := t.Ratings[i].Rating
		unique[s] = struct{}{}
	}
	for i := 0; i < len(orderedRatings); i++ {
		rating := orderedRatings[i]
		if _, ok := unique[rating]; ok {
			return rating, true
		}
	}
	return "Missing, NAR, or pre-2004 rating. Check URL.", false
}

func getMagicStrings(html []byte) (url.Values, error) {
	reader := bytes.NewReader(html)
	doc, _ := goquery.NewDocumentFromReader(reader)
	vs, _ := doc.Find("#__VIEWSTATE").Attr("value")
	vsg, _ := doc.Find("#__VIEWSTATEGENERATOR").Attr("value")
	ev, _ := doc.Find("#__EVENTVALIDATION").Attr("value")
	ms := map[string][]string{
		"__VIEWSTATE":          []string{vs},
		"__VIEWSTATEGENERATOR": []string{vsg},
		"__EVENTVALIDATION":    []string{ev},
	}
	return ms, nil
}

// Crawler is a type that embeds an http.Client and holds state information. 
type Crawler struct {
	http.Client
	magicStrings url.Values
	url          string
}

// NewCrawler returns a pointer to a new Crawler. 
func NewCrawler() (*Crawler, error) {
	jar, _ := cookiejar.New(nil)
	cl := http.Client{Jar: jar}
	c := &Crawler{
		Client:       cl,
		magicStrings: nil,
		url:          "https://app.mda.gov.sg/Classification/Search/Film/",
	}
	return c, nil
}

func (c *Crawler) doInit() error {
	r, err := c.Get(c.url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	html, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	ms, err := getMagicStrings(html)
	if err != nil {
		return err
	}
	c.magicStrings = ms
	return nil
}

func (c *Crawler) doSearch() error {
	ms := c.magicStrings
	vals := make(map[string][]string)
	for k, v := range ms {
		vals[k] = v
	}
	vals["chklstType$0"] = []string{"Feature"}
	vals["chklstType$2"] = []string{"Feature"}
	vals["chklstType$3"] = []string{"Serial"}
	vals["btnSearch"] = []string{"Search"}
	r, err := c.PostForm(c.url, vals)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	html, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	ms, err = getMagicStrings(html)
	if err != nil {
		return err
	}
	c.magicStrings = ms
	c.url = r.Request.URL.String()
	return nil
}

func (c *Crawler) requestPage(page int) error {
	vals := make(map[string][]string)
	for k, v := range c.magicStrings {
		vals[k] = v
	}
	vals["__EVENTTARGET"] = []string{"gvResult"}
	vals["__EVENTARGUMENT"] = []string{fmt.Sprint("Page$", page)}
	r, err := c.PostForm(c.url, vals)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	html, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	u := r.Request.URL.String()
	if u != c.url {
		msg := fmt.Sprintf("Post URL changed: %s (was: %s).", u, c.url)
		return errors.New(msg)
	}
	ms, err := getMagicStrings(html)
	if err != nil {
		return err
	}
	c.magicStrings = ms
	return nil
}

func checkResponse(html []byte) error {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return err
	}
	title := doc.Find("#lblTitle").Text()
	if title == "" {
		err = errors.New("title is the empty string")
		return err
	}
	return nil
}

func (c *Crawler) requestRow(row int) (*http.Response, error) {
	vals := make(map[string][]string)
	for k, v := range c.magicStrings {
		vals[k] = v
	}
	vals["__EVENTTARGET"] = []string{"gvResult"}
	vals["__EVENTARGUMENT"] = []string{fmt.Sprint("Title$", row)}
	resp, err := c.PostForm(c.url, vals)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Result is a type returned through a channel by the Crawl method of the Crawler type. It holds the HTML of a classification database title page.
type Result struct {
	URL  string // get-able URL of result page
	HTML []byte // html of the result page
	Page int // search result page the result was found on
	Row  int // search result row the result was found on
}

// The Crawl method takes a Job and two channels. The results channel is sent results as they are crawled. The jobs channal is sent jobs in the case of an error or they are done.
func (c *Crawler) Crawl(j Job, results chan<- Result, jobs chan<- Job) {
	// log.Print("Worker: doInit().")
	err := c.doInit()
	if err != nil {
		j.Error = err
		jobs <- j
		return
	}
	// log.Print("Worker: doSearch().")
	err = c.doSearch()
	if err != nil {
		j.Error = err
		jobs <- j
		return
	}
	// log.Print("Worker: seeking...")
	if j.page() > 1 {
		if j.page() > 11 {
			page := j.page()
			for i := 11; i < page; i = i + 10 {
				// log.Printf("Worker: Requesting page %v.", i)
				err = c.requestPage(i)
				if err != nil {
					j.Error = err
					jobs <- j
					return
				}
			}
			// log.Printf("Worker: Requesting page %v.", page)
			err = c.requestPage(page)
			if err != nil {
				j.Error = err
				jobs <- j
				return
			}
		}
	}
	// log.Print("Worker: Starting crawl loop.")
	done := false
	for !done {
		// log.Printf("Worker: Requesting page %v, row %v.", j.page(), j.row())
		resp, err := c.requestRow(j.row())
		if err != nil {
			j.Error = err
			jobs <- j
			return
		}
		html, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			j.Error = err
			jobs <- j
			return
		}
		err = checkResponse(html)
		if err != nil {
			j.Error = err
			jobs <- j
			return
		}
		result := Result{
			URL:  resp.Request.URL.String(),
			HTML: html,
			Page: j.page(),
			Row:  j.row(),
		}
		results <- result
		oldPage := j.page()
		j = j.next()
		done = j.IsDone()
		if done {
			jobs <- j
			return
		}
		newPage := j.page()
		needPage := oldPage != newPage
		if needPage {
			// log.Printf("Worker: Need page %v, requesting.", j.page())
			err = c.requestPage(j.page())
			if err != nil {
				j.Error = err
				jobs <- j
				return
			}
		}
	}
	// close channel?
}
