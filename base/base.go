package base

import (
	"encoding/json"
	"log"
	"strings"
)

var Color = struct {
	Black   string
	Blue    string
	Grey    string
	Red     string
	White   string
	Reset   string
	Reverse string
}{
	Black:   "\u001b[38;5;16m",
	Blue:    "\u001b[38;5;45m",
	Grey:    "\u001b[38;5;252m",
	Red:     "\u001b[38;5;42m",
	White:   "\u001b[38;5;255m",
	Reset:   "\u001b[0m",
	Reverse: "\u001b[7m",
}

func CheckErr(msg string, err error) {
	if err != nil {
		log.Println(Color.Red+msg+Color.Reset, err)
	}
}

func ParseDatabaseName(site string) string {
	return strings.ReplaceAll(strings.ReplaceAll(site, "://", "__"), ".", "_")
}

// Add underscrore to a string until it reachs 15 length
func AddUnderscore(str string) string {
	for len(str) < 15 {
		str += "_"
	}
	return str
}

// Parse Data From Frontend to Search
func ParseDataFromFrontend[T interface{}](results []interface{}) T {

	data := results[0].(map[string]interface{})

	// convert map to json
	jsonString, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	// T is your struct type
	var parsedData T

	// Convert json to struct
	json.Unmarshal(jsonString, &parsedData)
	log.Println(parsedData)

	return parsedData
}
