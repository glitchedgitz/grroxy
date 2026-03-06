package main

import (
	"fmt"
	"log"
	"regexp"

	"github.com/glitchedgitz/grroxy/internal/sdk"
)

var field = "resp"
var re *regexp.Regexp
var err error

func compileRegex() {
	if !isRegexp {
		search = regexp.QuoteMeta(search)
	}

	if !isCaseSensitive {
		search = "(?i)" + search
	}

	// If whole word search, add word boundaries (\b) to pattern
	if isWholeWord {
		search = `\b` + search + `\b`
	}
	log.Println("search", search)
	re, err = regexp.Compile(search)
	if err != nil {
		log.Println("Error compiling regex:", err)
		return
	}
}

func searchRecord(id string) {
	defer wg.Done()
	// log.Println(id)
	collection := sdk.CollectionSet[map[string]any](grroxydb, "_raw")
	data, err := collection.One(id)
	if err != nil {
		log.Println("Error reading record:", id, " ", err)
		return
	}

	// os.Exit(1)
	// Compile the regex

	d := data[field].(string)
	if d != "" {
		searches := re.FindAllString(d, -1)
		for _, search := range searches {
			fmt.Println(search)
		}
		// log.Println(id, " : ", searches)
	}

	if err != nil {
		log.Println(err)
	}
}
