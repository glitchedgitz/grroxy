///////////////////////////////////////
//
// Will be completed later
//
///////////////////////////////////////

package main

import (
	"flag"
	"log"
	"os"
	"sync"

	_ "github.com/glitchedgitz/grroxy-db/internal/logflags"
	"github.com/glitchedgitz/grroxy-db/internal/sdk"
	"github.com/glitchedgitz/grroxy-db/internal/types"
)

// Define your flags as global variables
var (
	isRegexp        bool
	isCaseSensitive bool
	isWholeWord     bool
	search          string
)

func init() {
	// Define the flags
	flag.BoolVar(&isRegexp, "regexp", false, "Treat search string as regular expression")
	flag.BoolVar(&isCaseSensitive, "caseSensitive", false, "Perform case-sensitive search")
	flag.BoolVar(&isWholeWord, "wholeWord", false, "Search for whole words only")
	flag.StringVar(&search, "search", "", "The string or pattern to search for")

	// Parse the flags
	flag.Parse()

	if search == "" {
		log.Println("Error: -search flag is required")
		// Output the usage info
		flag.Usage()
		os.Exit(1)
	}
}

var grroxydb *sdk.Client
var records chan types.ResponseList[map[string]any]
var wg sync.WaitGroup

func recordsManager() {
	for {
		record := <-records
		for _, r := range record.Items {
			go searchRecord(r["id"].(string))
		}
	}
}

var CONCURRENT = 50

func main() {

	compileRegex()
	grroxydb = sdk.NewClient(
		"http://127.0.0.1:8090",
		sdk.WithAdminEmailPassword("new@example.com", "1234567890"))

	collection := "_data"
	sortBy := "-created"
	filters :=
		"(resp.mime !~ 'video%' && resp.mime !~ 'audio%' && resp.mime !~ 'image%' && resp.mime !~ 'font%' && resp.mime !~ 'application/font%' && resp.mime !~ 'text/css%') && host ~ 'infojobs' && raw.resp ~ 'candidate-'"

	records = make(chan types.ResponseList[map[string]any])
	go recordsManager()
	page := 1
	total := 0
	TotalLen := 0
	for {
		response, err := grroxydb.List(collection, types.ParamsList{
			Page: page, Size: CONCURRENT, Sort: sortBy, Filters: filters,
		})

		if err != nil {
			log.Println(err)
		}

		TotalLen = response.TotalItems
		records <- response

		itemLen := len(response.Items)
		total += itemLen
		wg.Add(itemLen)
		if itemLen < CONCURRENT {
			break
		}
		page += 1
	}

	wg.Wait()

	log.Println("Total: ", total)
	log.Println("TotalLen: ", TotalLen)
}
