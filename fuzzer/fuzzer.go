package fuzzer

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/glitchedgitz/grroxy-db/logflags"
	"github.com/glitchedgitz/grroxy-db/rawhttp"
)

const ModeClusterBomb = "cluster_bomb"
const ModePitchFork = "pitch_fork"

type FuzzerConfig struct {
	Request     string
	Host        string
	Port        string
	UseTLS      bool
	UseHTTP2    bool // Enable HTTP/2 support
	Markers     map[string]string
	Mode        string
	Concurrency int
	Timeout     time.Duration
}

type FuzzerResult struct {
	Request  string
	Response string
	Time     time.Duration
	Markers  map[string]string
	Error    string
}

type Fuzzer struct {
	Config  FuzzerConfig
	Results chan any
	State   string
	mu      sync.RWMutex
	wg      sync.WaitGroup

	http        *rawhttp.Client
	files       map[string]*bufio.Reader
	fileHandles map[string]*os.File
}

func NewFuzzer(config FuzzerConfig) *Fuzzer {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	if config.Concurrency == 0 {
		config.Concurrency = 40
	}

	return &Fuzzer{Config: config,
		http:    rawhttp.NewClient(rawhttp.Config{Timeout: config.Timeout}),
		Results: make(chan any, config.Concurrency)}
}

func (f *Fuzzer) Fuzz() error {

	log.Printf("[fuzzer] Fuzz() called with config: host=%s port=%s useTLS=%v mode=%s concurrency=%d timeout=%s",
		f.Config.Host, f.Config.Port, f.Config.UseTLS, f.Config.Mode, f.Config.Concurrency, f.Config.Timeout)

	if f.Config.Request == "" {
		log.Println("[fuzzer] aborting: request is empty")
		return fmt.Errorf("request is empty")
	}

	if f.Config.Host == "" {
		log.Println("[fuzzer] aborting: host is empty")
		return fmt.Errorf("host is empty")
	}

	if f.Config.Port == "" {
		if f.Config.UseTLS {
			f.Config.Port = "443"
		} else {
			f.Config.Port = "80"
		}
		log.Printf("[fuzzer] port not set, defaulting to %s", f.Config.Port)
	}

	if f.files == nil {
		f.files = make(map[string]*bufio.Reader)
	}

	if f.fileHandles == nil {
		f.fileHandles = make(map[string]*os.File)
	}

	if f.Config.Mode == "" {
		f.Config.Mode = ModeClusterBomb
		log.Printf("[fuzzer] mode not set, defaulting to %s", f.Config.Mode)
	}

	if f.Config.Concurrency == 0 {
		f.Config.Concurrency = 10
		log.Printf("[fuzzer] concurrency not set, defaulting to %d", f.Config.Concurrency)
	}

	if f.Config.Mode != ModeClusterBomb && f.Config.Mode != ModePitchFork {
		log.Printf("[fuzzer] aborting: invalid mode %s", f.Config.Mode)
		return fmt.Errorf("invalid mode: %s", f.Config.Mode)
	}

	// defer close(f.results)

	for marker, wordlist := range f.Config.Markers {
		log.Printf("[fuzzer] opening wordlist for marker %s: %s", marker, wordlist)

		cwd, err := os.Getwd()
		if err != nil {
			log.Printf("[fuzzer] failed to get current working directory: %v", err)
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		log.Printf("[fuzzer] current working directory: %s", cwd)

		// time.Sleep(2 * time.Second)

		// // Debug: read and print the full wordlist content before using it
		// debugPath := path.Join(cwd, wordlist)

		// if b, err := os.ReadFile(debugPath); err != nil {
		// 	log.Printf("[fuzzer] failed to read full wordlist %s for debug: %v", debugPath, err)
		// } else {
		// 	log.Printf("[fuzzer] full content of wordlist %s:\n%s", debugPath, string(b))
		// }

		file, err := os.Open(wordlist)
		if err != nil {
			return fmt.Errorf("failed to open wordlist: %w", err)
		}
		f.files[marker] = bufio.NewReader(file)
		f.fileHandles[marker] = file
	}

	log.Printf("[fuzzer] initialized %d wordlists; starting fuzz loop", len(f.files))

outerLoop:
	for {

		markers := make(map[string]string)
		hitEOF := false

		if f.isStopped() {
			log.Println("[fuzzer] stop signal received before reading markers; breaking outer loop")
			break outerLoop
		}

		for marker := range f.files {
			markers[marker] = ""
		}

		for marker, reader := range f.files {
			if f.isStopped() {
				log.Println("[fuzzer] stop signal received during marker iteration; breaking outer loop")
				break outerLoop
			}

			word, err := reader.ReadString('\n')

			if err == io.EOF {
				log.Printf("[fuzzer] reached EOF for marker %s", marker)
				hitEOF = true
			}

			if err != nil && err != io.EOF {
				f.Stop()
				log.Println("[fuzzer] failed to read wordlist: %w", err)
				break outerLoop
			}

			if strings.HasSuffix(word, "\r\n") {
				word = word[:len(word)-2]
			} else if strings.HasSuffix(word, "\n") {
				word = word[:len(word)-1]
			}

			log.Println("[fuzzer] reading word: ", word)

			markers[marker] = word

			if f.Config.Mode == ModeClusterBomb {
				f.wg.Add(1)
				log.Printf("[fuzzer] dispatching request in cluster_bomb mode with markers=%v", markers)
				go f.SendRequest(markers)
			}
		}

		if f.Config.Mode == ModePitchFork {
			// Only process if we didn't hit EOF (all markers were read successfully)
			if !hitEOF {
				f.wg.Add(1)
				log.Printf("[fuzzer] dispatching request in pitch_fork mode with markers=%v", markers)
				go f.SendRequest(markers)
			}
		}

		if hitEOF {
			log.Println("[fuzzer] hit EOF on at least one wordlist; breaking outer loop")
			// f.Stop()
			break outerLoop
		}

		if f.isStopped() {
			log.Println("[fuzzer] stop signal received after iteration; breaking outer loop")
			break outerLoop
		}
	}

	// Wait for all pending goroutines to finish
	log.Println("[fuzzer] waiting for all in-flight requests to finish")
	f.wg.Wait()
	// Close the results channel so the receiver knows we're done
	log.Println("[fuzzer] all requests finished; closing results channel")
	close(f.Results)

	log.Println("[fuzzer] Fuzz() completed successfully")
	return nil

}

func (f *Fuzzer) Stop() {
	f.mu.Lock()
	f.State = "stopped"
	f.mu.Unlock()

	for _, file := range f.fileHandles {
		file.Close()
	}

	// close(f.results)
}

func (f *Fuzzer) isStopped() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.State == "stopped"
}

func (f *Fuzzer) ReplaceMarkers(markers map[string]string) string {
	request := f.Config.Request
	for marker, word := range markers {
		request = strings.ReplaceAll(request, marker, word)
	}
	return request
}

func (f *Fuzzer) SendRequest(markers map[string]string) {
	defer f.wg.Done()
	request := f.ReplaceMarkers(markers)
	req := rawhttp.Request{
		RawBytes: []byte(request),
		Host:     f.Config.Host,
		Port:     f.Config.Port,
		UseTLS:   f.Config.UseTLS,
		UseHTTP2: f.Config.UseHTTP2,
		Timeout:  f.Config.Timeout,
	}
	resp, err := f.http.Send(req)

	result := FuzzerResult{Request: request, Response: "", Time: 0, Markers: markers}
	if err == nil {
		result.Response = string(resp.RawBytes)
		result.Time = resp.ResponseTime
	} else {
		result.Response = err.Error()
	}

	f.Results <- result
}
