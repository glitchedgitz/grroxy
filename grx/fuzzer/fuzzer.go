package fuzzer

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
	_ "github.com/glitchedgitz/grroxy/internal/logflags"
)

const ModeClusterBomb = "cluster_bomb"
const ModePitchFork = "pitch_fork"

// markerSource provides sequential access to payload values.
type markerSource interface {
	// Next returns the next payload value. Returns io.EOF when exhausted.
	Next() (string, error)
	// Len returns the total number of payloads.
	Len() int
}

// fileSource reads payloads line-by-line from a file.
type fileSource struct {
	reader *bufio.Reader
	file   *os.File
	count  int
}

func (s *fileSource) Next() (string, error) {
	word, err := s.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	// Trim line endings
	if strings.HasSuffix(word, "\r\n") {
		word = word[:len(word)-2]
	} else if strings.HasSuffix(word, "\n") {
		word = word[:len(word)-1]
	}
	if err == io.EOF {
		return word, io.EOF
	}
	return word, nil
}

func (s *fileSource) Len() int { return s.count }

// sliceSource iterates over an in-memory slice of payloads (supports multi-line values).
type sliceSource struct {
	payloads []string
	index    int
}

func (s *sliceSource) Next() (string, error) {
	if s.index >= len(s.payloads) {
		return "", io.EOF
	}
	val := s.payloads[s.index]
	s.index++
	if s.index >= len(s.payloads) {
		return val, io.EOF
	}
	return val, nil
}

func (s *sliceSource) Len() int { return len(s.payloads) }

type FuzzerConfig struct {
	Request     string
	Host        string
	Port        string
	UseTLS      bool
	UseHTTP2    bool           // Enable HTTP/2 support
	Markers     map[string]any // marker -> string (file path) or []string (inline payloads)
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
	sources     map[string]markerSource
	fileHandles map[string]*os.File

	// Progress tracking using atomic operations (no mutex needed)
	totalRequests     int64
	completedRequests int64
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

	if f.sources == nil {
		f.sources = make(map[string]markerSource)
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

	// Set up sources from markers (string = file path, []string = inline payloads)
	for marker, value := range f.Config.Markers {
		switch v := value.(type) {
		case string:
			if v == "" {
				return fmt.Errorf("marker '%s' has empty file path", marker)
			}
			log.Printf("[fuzzer] opening wordlist for marker %s: %s", marker, v)
			file, err := os.Open(v)
			if err != nil {
				return fmt.Errorf("failed to open wordlist: %w", err)
			}
			// Count lines for progress tracking
			scanner := bufio.NewScanner(file)
			count := 0
			for scanner.Scan() {
				count++
			}
			file.Seek(0, 0)
			f.sources[marker] = &fileSource{reader: bufio.NewReader(file), file: file, count: count}
			f.fileHandles[marker] = file
		case []string:
			if len(v) == 0 {
				return fmt.Errorf("marker '%s' has empty payload list", marker)
			}
			log.Printf("[fuzzer] using %d inline payloads for marker %s", len(v), marker)
			f.sources[marker] = &sliceSource{payloads: v}
		case []interface{}:
			if len(v) == 0 {
				return fmt.Errorf("marker '%s' has empty payload list", marker)
			}
			payloads := make([]string, len(v))
			for i, item := range v {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("marker '%s' payload at index %d is not a string", marker, i)
				}
				payloads[i] = s
			}
			log.Printf("[fuzzer] using %d inline payloads for marker %s", len(payloads), marker)
			f.sources[marker] = &sliceSource{payloads: payloads}
		default:
			return fmt.Errorf("marker '%s' has invalid type: expected string (file path) or []string (payloads)", marker)
		}
	}

	if len(f.sources) == 0 {
		return fmt.Errorf("no markers configured: provide markers as string (file path) or array (inline payloads)")
	}

	log.Printf("[fuzzer] initialized %d marker sources; starting fuzz loop", len(f.sources))

	// Calculate total requests for progress tracking
	totalRequests := f.calculateTotalRequests()
	f.SetTotalRequests(totalRequests)
	log.Printf("[fuzzer] total requests to process: %d", totalRequests)

outerLoop:
	for {

		markers := make(map[string]string)
		hitEOF := false

		if f.isStopped() {
			log.Println("[fuzzer] stop signal received before reading markers; breaking outer loop")
			break outerLoop
		}

		for marker := range f.sources {
			markers[marker] = ""
		}

		for marker, src := range f.sources {
			if f.isStopped() {
				log.Println("[fuzzer] stop signal received during marker iteration; breaking outer loop")
				break outerLoop
			}

			word, err := src.Next()

			if err == io.EOF {
				log.Printf("[fuzzer] reached EOF for marker %s", marker)
				hitEOF = true
			}

			if err != nil && err != io.EOF {
				f.Stop()
				log.Println("[fuzzer] failed to read source: %w", err)
				break outerLoop
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
			// Dispatch if we have valid marker values (EOF may arrive with the last value)
			hasValues := false
			for _, v := range markers {
				if v != "" {
					hasValues = true
					break
				}
			}
			if hasValues {
				f.wg.Add(1)
				log.Printf("[fuzzer] dispatching request in pitch_fork mode with markers=%v", markers)
				go f.SendRequest(markers)
			}
		}

		if hitEOF {
			log.Println("[fuzzer] hit EOF on at least one source; breaking outer loop")
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

	// Increment completed requests atomically
	f.IncrementCompleted()
}

// IncrementCompleted increments the completed requests counter atomically
func (f *Fuzzer) IncrementCompleted() {
	atomic.AddInt64(&f.completedRequests, 1)
}

// GetProgress returns the current progress (completed, total)
func (f *Fuzzer) GetProgress() (int, int) {
	completed := atomic.LoadInt64(&f.completedRequests)
	total := atomic.LoadInt64(&f.totalRequests)
	return int(completed), int(total)
}

// SetTotalRequests sets the total number of requests atomically
func (f *Fuzzer) SetTotalRequests(total int) {
	atomic.StoreInt64(&f.totalRequests, int64(total))
}

// calculateTotalRequests calculates the total number of requests based on source sizes and mode
func (f *Fuzzer) calculateTotalRequests() int {
	// Calculate total based on mode
	if f.Config.Mode == ModeClusterBomb {
		total := 1
		for _, src := range f.sources {
			total *= src.Len()
		}
		return total
	} else if f.Config.Mode == ModePitchFork {
		if len(f.sources) == 0 {
			return 0
		}
		min := -1
		for _, src := range f.sources {
			if min == -1 || src.Len() < min {
				min = src.Len()
			}
		}
		return min
	}

	return 0
}
