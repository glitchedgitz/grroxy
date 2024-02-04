package base

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
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

func FormatNumericID(number, width int) string {
	// Convert the number to a string
	numStr := fmt.Sprintf("%d", number)

	// Calculate the number of underscores needed for padding
	underscoreCount := width - len(numStr)

	// Create the padded string with underscores
	paddedStr := strings.Repeat("_", underscoreCount) + numStr

	return paddedStr
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

func ResponseToByte(resp *http.Response) ([]byte, error) {

	body, err := io.ReadAll(resp.Body)
	// resp.Body.Close()

	if err != nil {
		return []byte(""), fmt.Errorf("failed to read the response body: %w", err)
	}

	// Create a new response without the chunked encoding information
	newResp := &http.Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		Header:        resp.Header,
		ContentLength: int64(len(body)),
		Body:          io.NopCloser(bytes.NewReader(body)),
		Request:       resp.Request,
	}

	respBytes, err := httputil.DumpResponse(newResp, true)
	if err != nil {
		return []byte(""), err
	}

	return respBytes, nil
}

func ResponseToString(resp *http.Response) (string, error) {
	// Read the response body

	respBytes, err := ResponseToByte(resp)
	CheckErr("[ResponseToString]: ", err)

	return string(respBytes), nil
}
