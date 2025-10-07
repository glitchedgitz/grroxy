package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/jpillora/go-tld"
	"golang.org/x/net/html"
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

func CalculateTime(old, new time.Time) string {

	timeDifference := new.Sub(old)

	days := int(timeDifference.Hours()) / 24
	hours := int(timeDifference.Hours()) % 24
	minutes := int(timeDifference.Minutes()) % 60
	seconds := int(timeDifference.Seconds()) % 60
	milliseconds := int(timeDifference.Nanoseconds()/1e6) % 1000

	// Construct the formatted string
	var formattedTime string

	if days > 0 {
		formattedTime += fmt.Sprintf("%dd ", days)
	}
	if hours > 0 {
		formattedTime += fmt.Sprintf("%dh ", hours)
	}
	if minutes > 0 {
		formattedTime += fmt.Sprintf("%dm ", minutes)
	}
	if seconds > 0 {
		formattedTime += fmt.Sprintf("%ds ", seconds)
	}
	if milliseconds > 0 {
		formattedTime += fmt.Sprintf("%dms", milliseconds)
	}

	// Print the formatted time difference
	return formattedTime
}

func CheckErr(msg string, err error) {
	if err != nil {
		log.Println(Color.Red+msg+Color.Reset, err)
	}
}

func FormatNumericID(number float64, width int) string {
	// Convert the number to a string
	numStr := fmt.Sprintf("%g", number)

	// Calculate the number of underscores needed for padding
	underscoreCount := width - len(numStr)

	// Create the padded string with underscores
	paddedStr := strings.Repeat("_", underscoreCount) + numStr

	return paddedStr
}

func FormatStringID(str string, width int) string {

	// Calculate the number of underscores needed for padding
	underscoreCount := width - len(str)

	// Create the padded string with underscores
	paddedStr := strings.Repeat("_", underscoreCount) + str

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

	// Read the body once first
	originalBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte(""), fmt.Errorf("failed to read the response body: %w", err)
	}

	// Check for compression and decompress if needed
	contentEncoding := resp.Header.Get("Content-Encoding")
	var bodyReader io.Reader
	var decompressed bool

	switch strings.ToLower(contentEncoding) {
	case "gzip", "x-gzip":
		gzReader, err := gzip.NewReader(bytes.NewReader(originalBody))
		if err != nil {
			// If decompression fails, use original body
			bodyReader = bytes.NewReader(originalBody)
			decompressed = false
		} else {
			bodyReader = gzReader
			decompressed = true
		}
	case "br", "brotli":
		bodyReader = brotli.NewReader(bytes.NewReader(originalBody))
		decompressed = true
	default:
		bodyReader = bytes.NewReader(originalBody)
		decompressed = false
	}

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		// If reading decompressed data fails, fall back to original
		body = originalBody
		decompressed = false
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

	// Remove Content-Encoding header only if we successfully decompressed
	if decompressed {
		// newResp.Header.Del("Content-Encoding")
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

func SmartSort(s string) string {
	u, err := tld.Parse(s)
	if err != nil {
		log.Println(err)
		return strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	}
	arr := strings.Split(u.Subdomain, ".")
	arr = append(arr, u.TLD)
	arr = append(arr, u.Domain)

	arr2 := []string{}
	for i := len(arr); i > 0; i-- {
		arr2 = append(arr2, arr[i-1])
	}

	return strings.Join(arr2, ".")
}

func ExtractTitle(respByte []byte) (string, string) {

	title := ""
	favicon := ""

	z := html.NewTokenizer(bytes.NewReader(respByte))

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		t := z.Token()

		if t.Type == html.StartTagToken {
			if t.Data == "title" {
				if z.Next() == html.TextToken {
					title = strings.TrimSpace(z.Token().Data)
					break
				}
			}
			// else if t.Data == "link" {
			// 	if z.Next() == html.TextToken {
			// 		favicon = strings.TrimSpace(z.Token().Data)
			// 		break
			// 	}
			// }
		}
	}
	return title, favicon
}

func StructToMap(s any, tag string) map[string]any {

	log.Println("[StructToMap] s:", s)
	log.Println("[StructToMap] tag:", tag)

	result := make(map[string]any)
	val := reflect.ValueOf(s).Elem() // Get the value of the struct
	typ := val.Type()                // Get the type of the struct

	log.Println("[StructToMap] val:", val)
	log.Println("[StructToMap] typ:", typ)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldTag := typ.Field(i).Tag.Get(tag) // Get the value of the specified tag
		fieldTag = strings.Split(fieldTag, ",")[0]
		if field.Kind() == reflect.Struct {
			result[fieldTag] = StructToMap(field.Addr().Interface(), tag)
		} else {
			result[fieldTag] = field.Interface()
		}
		log.Println("[StructToMap] key:", fieldTag, "value:", result[fieldTag])
	}

	log.Println("[StructToMap] result:", result)
	return result
}

func StructToJsonTOMap(s any) (map[string]any, error) {
	// Convert struct to JSON
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	// Convert JSON to map
	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func StructToMapExtact(s any) map[string]any {
	result := make(map[string]interface{})
	val := reflect.ValueOf(s)
	typ := reflect.TypeOf(s)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldName := typ.Field(i).Name
		result[fieldName] = field.Interface()
	}

	return result
}

func ExtractValueFromMap(d *map[string]any, givenKey string) (any, error) {
	if strings.Contains(givenKey, ".") {
		parts := strings.Split(givenKey, ".")
		currentMap := *d

		for i, part := range parts {
			value, found := currentMap[part]
			if !found {
				return nil, fmt.Errorf("key %s not found", part)
			}

			if i == len(parts)-1 {
				return value, nil
			}

			// Check if the next level is also a map
			switch converted := value.(type) {
			case map[string]any:
				currentMap = converted
			case map[string]string:
				// Special case to handle map[string]string
				if i == len(parts)-2 { // Next is the last part
					lastValue, lastFound := converted[parts[i+1]]
					if !lastFound {
						return nil, fmt.Errorf("key %s not found", parts[i+1])
					}
					return lastValue, nil
				}
				return nil, errors.New("map[string]string encountered before last key part")
			default:
				return nil, fmt.Errorf("expected a map at key %s, but found type %T", part, value)
			}
		}
	}

	// If no dots in key, perform a simple lookup
	value, found := (*d)[givenKey]
	if !found {
		return nil, fmt.Errorf("key %s not found", givenKey)
	}
	return value, nil
}

// ReplaceString processes the input `value` string, replacing occurrences of `search` with `replace`.
// If `regex` is true, `search` is treated as a regular expression.
func FindAndReplaceAll(value, search, replace string, regex bool) (string, error) {
	if regex {
		// Compile the regex pattern from the search string
		re, err := regexp.Compile(search)
		if err != nil {
			return "", fmt.Errorf("[FindAndReplaceAll] invalid regex pattern: %w", err)
		}
		// Replace all occurrences using the compiled regex
		return re.ReplaceAllString(value, replace), nil
	}
	// Perform a simple string replacement if regex is not requested
	return strings.ReplaceAll(value, search, replace), nil
}

func ArrayContains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func ChangeCase(s string) string {
	s_array := strings.Split(s, "_")
	s_array_length := len(s_array)
	s = ""
	for i, val := range s_array {
		s += strings.Title(val)
		if i < s_array_length-1 {
			s += "-"
		}
	}
	return s
}
