package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/glitchedgitz/grroxy-db/grx/fuzzer"
	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
	"github.com/spf13/cobra"
)

var (
	// Request parameters
	request     string
	requestFile string
	host        string
	port        string
	useTLS      bool
	useHTTP2    bool

	// Fuzzer parameters
	markers     map[string]string
	mode        string
	concurrency int
	timeout     float64

	// Output parameters
	outputFile string
	verbose    bool
)

type FuzzerResult struct {
	Request      string            `json:"request"`
	Response     string            `json:"response"`
	ResponseTime float64           `json:"response_time_ms"`
	Markers      map[string]string `json:"markers"`
	Error        string            `json:"error,omitempty"`
	StatusCode   int               `json:"status_code,omitempty"`
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "grx-fuzzer",
		Short: "grroxy HTTP/HTTP2 fuzzer - standalone fuzzing tool",
		Long: `grx-fuzzer is a standalone fuzzing tool that supports both HTTP/1.x and HTTP/2.
It allows you to fuzz web applications directly from the command line.`,
		Example: `  # Basic HTTP/1.1 fuzzing
  grx-fuzzer --host example.com --request "GET /ยงPATHยง HTTP/1.1\r\nHost: example.com\r\n\r\n" \
    --marker "ยงPATHยง=wordlist.txt" --output results.json

  # HTTP/2 fuzzing with authentication
  grx-fuzzer --host api.example.com --tls --http2 \
    --request-file request.txt \
    --marker "ยงTOKENยง=tokens.txt" \
    --concurrency 50 --output results.json

  # POST request with multiple markers
  grx-fuzzer --host auth.example.com --tls \
    --request "POST /login HTTP/1.1\r\nHost: auth.example.com\r\nContent-Type: application/json\r\n\r\n{\"user\":\"ยงUSERยง\",\"pass\":\"ยงPASSยง\"}" \
    --marker "ยงUSERยง=users.txt" \
    --marker "ยงPASSยง=passwords.txt" \
    --mode pitch_fork --output results.json`,
		Run: runFuzzer,
	}

	// Request flags
	rootCmd.Flags().StringVar(&request, "request", "", "Raw HTTP request (use \\r\\n for line breaks)")
	rootCmd.Flags().StringVar(&requestFile, "request-file", "", "File containing raw HTTP request")
	rootCmd.Flags().StringVar(&host, "host", "", "Target host (required)")
	rootCmd.Flags().StringVar(&port, "port", "", "Target port (default: 80 for HTTP, 443 for HTTPS)")
	rootCmd.Flags().BoolVar(&useTLS, "tls", false, "Use TLS/HTTPS")
	rootCmd.Flags().BoolVar(&useHTTP2, "http2", false, "Use HTTP/2 protocol (requires --tls)")

	// Fuzzer flags
	rootCmd.Flags().StringToStringVar(&markers, "marker", map[string]string{}, "Fuzzing markers (format: MARKER=wordlist.txt, can be specified multiple times)")
	rootCmd.Flags().StringVar(&mode, "mode", "cluster_bomb", "Fuzzing mode: cluster_bomb or pitch_fork")
	rootCmd.Flags().IntVar(&concurrency, "concurrency", 40, "Number of concurrent requests")
	rootCmd.Flags().Float64Var(&timeout, "timeout", 10, "Request timeout in seconds")

	// Output flags
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for results (JSON format)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Mark required flags
	rootCmd.MarkFlagRequired("host")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runFuzzer(cmd *cobra.Command, args []string) {
	// Validate inputs
	if request == "" && requestFile == "" {
		log.Fatal("Error: Either --request or --request-file must be specified")
	}

	if request != "" && requestFile != "" {
		log.Fatal("Error: Cannot specify both --request and --request-file")
	}

	if useHTTP2 && !useTLS {
		log.Fatal("Error: HTTP/2 requires TLS (use --tls flag)")
	}

	if len(markers) == 0 {
		log.Fatal("Error: At least one marker must be specified with --marker")
	}

	// Read request from file if specified
	if requestFile != "" {
		data, err := os.ReadFile(requestFile)
		if err != nil {
			log.Fatalf("Error reading request file: %v", err)
		}
		request = string(data)
	}

	// Set default port
	if port == "" {
		if useTLS {
			port = "443"
		} else {
			port = "80"
		}
	}

	// Print configuration
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              grroxy HTTP/HTTP2 Fuzzer                          ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Printf("\n[*] Target:        %s:%s\n", host, port)
	fmt.Printf("[*] Protocol:      HTTP/%s over %s\n", protocolVersion(), tlsStatus())
	fmt.Printf("[*] Mode:          %s\n", mode)
	fmt.Printf("[*] Concurrency:   %d\n", concurrency)
	fmt.Printf("[*] Timeout:       %.1fs\n", timeout)
	fmt.Printf("[*] Markers:       %d\n", len(markers))
	for marker, wordlist := range markers {
		fmt.Printf("    - %s => %s\n", marker, wordlist)
	}
	if outputFile != "" {
		fmt.Printf("[*] Output:        %s\n", outputFile)
	}
	fmt.Println()

	// Create fuzzer config
	config := fuzzer.FuzzerConfig{
		Request:     request,
		Host:        host,
		Port:        port,
		UseTLS:      useTLS,
		UseHTTP2:    useHTTP2,
		Markers:     markers,
		Mode:        mode,
		Concurrency: concurrency,
		Timeout:     time.Duration(timeout * float64(time.Second)),
	}

	// Create fuzzer instance
	f := fuzzer.NewFuzzer(config)

	// Prepare output
	var results []FuzzerResult
	var outputChan chan FuzzerResult
	if outputFile != "" {
		outputChan = make(chan FuzzerResult, concurrency)
	}

	// Start result processor
	resultCount := 0
	errorCount := 0
	go func() {
		for result := range f.Results {
			fuzzerResult, ok := result.(fuzzer.FuzzerResult)
			if !ok {
				continue
			}

			resultCount++

			// Parse status code from response
			statusCode := 0
			parsed := rawhttp.ParseResponse([]byte(fuzzerResult.Response))
			statusCode = parsed.Status

			// Create output result
			outputResult := FuzzerResult{
				Request:      fuzzerResult.Request,
				Response:     fuzzerResult.Response,
				ResponseTime: float64(fuzzerResult.Time.Milliseconds()),
				Markers:      fuzzerResult.Markers,
				StatusCode:   statusCode,
			}

			if fuzzerResult.Error != "" {
				outputResult.Error = fuzzerResult.Error
				errorCount++
			}

			// Save to output channel
			if outputChan != nil {
				outputChan <- outputResult
			}

			// Print result if verbose
			if verbose {
				printResult(resultCount, &fuzzerResult, statusCode)
			} else {
				// Print progress
				if resultCount%10 == 0 {
					fmt.Printf("\r[*] Processed: %d requests (errors: %d)", resultCount, errorCount)
				}
			}
		}

		if outputChan != nil {
			close(outputChan)
		}
	}()

	// Collect results for output file
	if outputFile != "" {
		go func() {
			for result := range outputChan {
				results = append(results, result)
			}
		}()
	}

	// Start fuzzing
	fmt.Println("[+] Starting fuzzer...")
	startTime := time.Now()

	err := f.Fuzz()
	if err != nil {
		log.Fatalf("\n[!] Fuzzing error: %v", err)
	}

	duration := time.Since(startTime)

	// Print summary
	fmt.Printf("\n\n╔════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                      Fuzzing Complete                          ║\n")
	fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n[+] Total Requests:   %d\n", resultCount)
	fmt.Printf("[+] Errors:           %d\n", errorCount)
	fmt.Printf("[+] Success Rate:     %.2f%%\n", float64(resultCount-errorCount)/float64(resultCount)*100)
	fmt.Printf("[+] Duration:         %s\n", duration.Round(time.Millisecond))
	fmt.Printf("[+] Requests/sec:     %.2f\n", float64(resultCount)/duration.Seconds())

	// Save results to file
	if outputFile != "" {
		// Wait a bit for all results to be collected
		time.Sleep(100 * time.Millisecond)

		// Create output directory if needed
		outputDir := filepath.Dir(outputFile)
		if outputDir != "." && outputDir != "" {
			os.MkdirAll(outputDir, 0755)
		}

		// Save as JSON
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			log.Fatalf("\n[!] Error marshaling results: %v", err)
		}

		err = os.WriteFile(outputFile, data, 0644)
		if err != nil {
			log.Fatalf("\n[!] Error writing output file: %v", err)
		}

		fmt.Printf("\n[+] Results saved to: %s\n", outputFile)
	}

	fmt.Println()
}

func printResult(count int, result *fuzzer.FuzzerResult, statusCode int) {
	fmt.Printf("\n╔═══════════════════ Result #%d ═══════════════════╗\n", count)

	// Print markers
	fmt.Println("Markers:")
	for k, v := range result.Markers {
		fmt.Printf("  %s = %s\n", k, v)
	}

	// Print status
	if result.Error != "" {
		fmt.Printf("Status:   ERROR\n")
		fmt.Printf("Error:    %s\n", result.Error)
	} else {
		fmt.Printf("Status:   %d\n", statusCode)
		fmt.Printf("Time:     %s\n", result.Time)
		fmt.Printf("Response: %d bytes\n", len(result.Response))
	}

	fmt.Println("╚════════════════════════════════════════════════════╝")
}

func protocolVersion() string {
	if useHTTP2 {
		return "2.0"
	}
	return "1.1"
}

func tlsStatus() string {
	if useTLS {
		return "TLS"
	}
	return "plaintext"
}
