package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glitchedgitz/dadql/dadql"
	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
	"github.com/spf13/cobra"
)

type Param struct {
	Value string `json:"value"`
	Key   string `json:"key"`
	Type  string `json:"type"`
}

type ParsedURL struct {
	Url      string             `json:"url"`
	Scheme   string             `json:"scheme"`
	Host     string             `json:"host"`
	Port     string             `json:"port"`
	TLS      bool               `json:"tls"`
	Path     string             `json:"path"`
	RawPath  string             `json:"raw_path,omitempty"`
	Query    string             `json:"query,omitempty"`
	Params   map[string][]Param `json:"params,omitempty"`
	Fragment string             `json:"fragment,omitempty"`
	User     string             `json:"user,omitempty"`
	Ext      string             `json:"ext,omitempty"`
	// Response fields (only populated when alive flag is set)
	Resp *ResponseData `json:"resp,omitempty"`
}

type ResponseData struct {
	Status  int               `json:"status"`
	Length  int64             `json:"length"`
	Time    int64             `json:"time"`
	Mime    string            `json:"mime,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

var (
	filter         string
	responseFilter string
	jsonOutput     bool
	alive          bool
	concurrency    int
	timeout        float64
	delay          int
	method         string
	verbose        bool
	showStatus     bool
)

var rootCmd = &cobra.Command{
	Use:   "grxp",
	Short: "GRXP parses URLs from stdin",
	Long: `GRXP reads URLs from stdin (one per line) and outputs URLs (or JSON with -j flag).
Can probe URLs with -a flag to check if they're alive.

Example:
  cat urls.txt | grxp
  echo "https://example.com/path?key=value" | grxp
  cat urls.txt | grxp -j
  cat urls.txt | grxp -a
  cat urls.txt | grxp -a -j
  cat urls.txt | grxp -f "scheme = 'https' && tls = true"
  cat urls.txt | grxp -a -f "ext = 'json'" -rf "resp.status = 200"

Filter Examples (dadql syntax):
  # URL filters (applied before probing)
  cat urls.txt | grxp -f "scheme = 'https'"
  cat urls.txt | grxp -f "tls = true"
  cat urls.txt | grxp -f "host ~ 'example'"
  cat urls.txt | grxp -f "path ~ '/api'"
  cat urls.txt | grxp -f "port = '443'"
  cat urls.txt | grxp -f "ext = 'json'"
  
  # Response filters (applied after probing, only with -a)
  cat urls.txt | grxp -a -r "resp.status = 200"
  cat urls.txt | grxp -a -r "resp.status >= 200 && resp.status < 300"
  cat urls.txt | grxp -a -r "resp.length > 1000"
  cat urls.txt | grxp -a -r "resp.body ~ 'admin'"
  
  # Combined: filter URLs first, then filter responses
  cat urls.txt | grxp -a -f "ext = 'json'" -r "resp.status = 200"

Filter Operators:
  =, ==    Equal
  !=       Not equal
  ~        Like/Contains (use %% as wildcard, e.g., '%%example%%')
  !~       Not like
  >, >=    Greater than (or equal)
  <, <=    Less than (or equal)
  AND       AND
  OR       OR
  NOT      NOT

Available fields:
  url, scheme, host, port, tls, path, raw_path, query, fragment, user, ext, params.*
  (with -a flag: resp.status, resp.length, resp.time, resp.mime, resp.headers.*, resp.body)`,
	Run: run,
}

func init() {
	rootCmd.Flags().StringVarP(&filter, "filter", "f", "", "dadql filter expression for URL fields (e.g., 'scheme = \"https\"' or 'ext = \"json\"')")
	rootCmd.Flags().StringVarP(&responseFilter, "response-filter", "r", "", "dadql filter expression for response fields (only with -a, e.g., 'resp.status = 200')")
	rootCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "output as JSON (default: print URL only)")
	rootCmd.Flags().BoolVarP(&alive, "alive", "a", false, "probe URLs and only output those that respond")
	rootCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 50, "Number of concurrent requests (only with -a)")
	rootCmd.Flags().Float64VarP(&timeout, "timeout", "t", 10, "Timeout in seconds per request (only with -a)")
	rootCmd.Flags().IntVarP(&delay, "delay", "d", 0, "Delay between requests in milliseconds (only with -a)")
	rootCmd.Flags().StringVarP(&method, "method", "m", "GET", "HTTP method to use (GET, HEAD) (only with -a)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show errors (only with -a)")
	rootCmd.Flags().BoolVarP(&showStatus, "status-code", "s", false, "Show status code (only with -a)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	if alive {
		runWithProbe()
	} else {
		runWithoutProbe()
	}
}

func runWithoutProbe() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parsed := parseURL(line)

		// Check filter if provided
		if filter != "" {
			if !matchesFilter(parsed) {
				continue
			}
		}

		if jsonOutput {
			jsonData, err := json.Marshal(parsed)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				continue
			}
			fmt.Println(string(jsonData))
		} else {
			fmt.Println(parsed.Url)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}

func runWithProbe() {
	client := rawhttp.NewClient(rawhttp.Config{
		Timeout:            time.Duration(timeout * float64(time.Second)),
		InsecureSkipVerify: true,
	})

	urls := make(chan string, concurrency)
	var wg sync.WaitGroup
	var outputMu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rawURL := range urls {
				parsed := parseURL(rawURL)

				// Pre-filter by URL-only fields if filter is provided
				// This avoids unnecessary HTTP requests
				if filter != "" {
					if !matchesFilter(parsed) {
						if delay > 0 {
							time.Sleep(time.Duration(delay) * time.Millisecond)
						}
						continue
					}
				}

				respData, status, isAlive := checkURL(client, rawURL, &parsed)

				if isAlive {
					parsed.Resp = respData

					// Check response filter if provided
					if !matchesResponseFilter(parsed) {
						if delay > 0 {
							time.Sleep(time.Duration(delay) * time.Millisecond)
						}
						continue
					}

					outputMu.Lock()
					if jsonOutput {
						jsonData, err := json.Marshal(parsed)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
						} else {
							fmt.Println(string(jsonData))
						}
					} else {
						if showStatus {
							fmt.Printf("[%d] %s\n", status, rawURL)
						} else {
							fmt.Println(rawURL)
						}
					}
					outputMu.Unlock()
				} else if verbose {
					outputMu.Lock()
					fmt.Fprintf(os.Stderr, "[ERR] %s\n", rawURL)
					outputMu.Unlock()
				}

				if delay > 0 {
					time.Sleep(time.Duration(delay) * time.Millisecond)
				}
			}
		}()
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			urls <- line
		}
	}
	close(urls)

	wg.Wait()

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}

func parseURL(rawURL string) ParsedURL {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ParsedURL{
			Url: rawURL,
		}
	}

	parsed := ParsedURL{
		Url:      rawURL,
		Scheme:   u.Scheme,
		Host:     u.Hostname(),
		Port:     u.Port(),
		Path:     u.Path,
		RawPath:  u.RawPath,
		Query:    u.RawQuery,
		Fragment: u.Fragment,
		TLS:      u.Scheme == "https",
		Ext:      getExtension(u.Path),
	}

	// Only populate Params if there are query parameters
	if u.RawQuery != "" {
		params := make(map[string][]Param)
		for key, values := range u.Query() {
			for _, value := range values {
				paramType := detectType(value)
				params[key] = append(params[key], Param{
					Value: value,
					Key:   key,
					Type:  paramType,
				})
			}
		}
		parsed.Params = params
	}

	if u.User != nil {
		parsed.User = u.User.Username()
	}

	return parsed
}

func detectType(value string) string {
	if value == "" {
		return "string"
	}

	// Try to parse as boolean
	if value == "true" || value == "false" {
		return "boolean"
	}

	// Try to parse as integer (check if it contains a decimal point first)
	if !strings.Contains(value, ".") {
		if _, err := strconv.ParseInt(value, 10, 64); err == nil {
			return "integer"
		}
	}

	// Try to parse as float
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return "float"
	}

	return "string"
}

func getExtension(path string) string {
	if path == "" {
		return ""
	}

	// Remove query string and fragment if present
	path = strings.Split(path, "?")[0]
	path = strings.Split(path, "#")[0]

	// Get the last part of the path
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}

	lastPart := parts[len(parts)-1]
	if lastPart == "" {
		return ""
	}

	// Check if it has an extension
	dotIndex := strings.LastIndex(lastPart, ".")
	if dotIndex == -1 || dotIndex == len(lastPart)-1 {
		return ""
	}

	ext := strings.ToLower(lastPart[dotIndex+1:])
	// Remove any query parameters from extension
	ext = strings.Split(ext, "?")[0]
	ext = strings.Split(ext, "#")[0]

	return ext
}

func matchesFilter(parsed ParsedURL) bool {
	filterToUse := filter
	if filterToUse == "" {
		return true
	}

	// Convert ParsedURL to map[string]any for dadql
	data := parsedToMap(parsed)

	match, err := dadql.Filter(data, filterToUse)
	if err != nil {
		// fmt.Fprintf(os.Stderr, "[FILTER ERR] %s: %v\n", parsed.Url, err)
		return false
	}

	return match
}

func matchesResponseFilter(parsed ParsedURL) bool {
	if responseFilter == "" {
		return true
	}

	// Convert ParsedURL to map[string]any for dadql
	data := parsedToMap(parsed)

	match, err := dadql.Filter(data, responseFilter)
	if err != nil {
		// fmt.Fprintf(os.Stderr, "[RESPONSE FILTER ERR] %s: %v\n", parsed.Url, err)
		return false
	}

	return match
}

func parsedToMap(parsed ParsedURL) map[string]any {
	data := map[string]any{
		"url":      parsed.Url,
		"scheme":   parsed.Scheme,
		"host":     parsed.Host,
		"port":     parsed.Port,
		"tls":      parsed.TLS,
		"path":     parsed.Path,
		"query":    parsed.Query,
		"fragment": parsed.Fragment,
	}

	if parsed.Ext != "" {
		data["ext"] = parsed.Ext
	}

	if parsed.RawPath != "" {
		data["raw_path"] = parsed.RawPath
	}

	if parsed.User != "" {
		data["user"] = parsed.User
	}

	if len(parsed.Params) > 0 {
		// Convert params to a format that's easier to query
		paramsMap := make(map[string]any)
		for key, params := range parsed.Params {
			paramsMap[key] = params
		}
		data["params"] = paramsMap
	}

	// Add response data if available
	if parsed.Resp != nil {
		data["resp"] = map[string]any{
			"status":  parsed.Resp.Status,
			"length":  parsed.Resp.Length,
			"time":    parsed.Resp.Time,
			"mime":    parsed.Resp.Mime,
			"headers": parsed.Resp.Headers,
			"body":    parsed.Resp.Body,
		}
	}

	return data
}

func checkURL(client *rawhttp.Client, rawURL string, parsed *ParsedURL) (*ResponseData, int, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, false
	}

	useTLS := u.Scheme == "https"
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if useTLS {
			port = "443"
		} else {
			port = "80"
		}
	}

	path := u.RequestURI()
	if path == "" {
		path = "/"
	}

	rawReq := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\nUser-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36\r\nAccept: */*\r\n\r\n",
		method, path, host)

	resp, err := client.Send(rawhttp.Request{
		RawBytes: []byte(rawReq),
		Host:     host,
		Port:     port,
		UseTLS:   useTLS,
	})
	if err != nil {
		return nil, 0, false
	}

	parsedResp := rawhttp.ParseResponse(resp.RawBytes)
	if parsedResp.Status == 0 {
		return nil, 0, false
	}

	// Build response headers map
	respHeaders := make(map[string]string)
	httpHdr := http.Header{}
	for _, header := range parsedResp.Headers {
		if len(header) >= 2 {
			key := strings.TrimSuffix(header[0], ":")
			val := strings.TrimSpace(header[1])
			httpHdr.Set(key, val)
			respHeaders[key] = val
		}
	}

	// Get content length
	var contentLen int64 = int64(len(parsedResp.Body))
	if clStr, ok := rawhttp.GetHeaderValue(parsedResp.Headers, "content-length"); ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(clStr), 10, 64); err == nil {
			contentLen = n
		}
	}

	respData := &ResponseData{
		Status:  parsedResp.Status,
		Length:  contentLen,
		Time:    resp.ResponseTime.Milliseconds(),
		Mime:    httpHdr.Get("Content-Type"),
		Headers: respHeaders,
		Body:    parsedResp.Body,
	}

	return respData, parsedResp.Status, true
}
