package fuzzer_test

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/glitchedgitz/grroxy/grx/fuzzer"
)

func TestFuzzer(t *testing.T) {
	t.Log("[fuzzer] starting test")

	var wg sync.WaitGroup

	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET /FUZZ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
		Markers: map[string]any{
			"FUZZ": "./Uw8sq8xo2u3Tw9AefgcgYpod",
		},
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range f.Results {
			// fmt.Println(result.(fuzzer.FuzzerResult).Request)
			// fmt.Println(result.(fuzzer.FuzzerResult).Response)
			// fmt.Println(result.(fuzzer.FuzzerResult).Time)
			fmt.Println(result.(fuzzer.FuzzerResult).Markers)
		}
	}()

	t.Log("[fuzzer] results: ", f.Results)
	t.Log("[fuzzer] state: ", f.State)
	t.Log("[fuzzer] config: ", f.Config)

	err := f.Fuzz()
	if err != nil {
		t.Fatalf("failed to fuzz: %v", err)
	}

	wg.Wait()

}

// collectResults drains f.Results and returns all FuzzerResults.
func collectResults(f *fuzzer.Fuzzer) []fuzzer.FuzzerResult {
	var results []fuzzer.FuzzerResult
	for r := range f.Results {
		results = append(results, r.(fuzzer.FuzzerResult))
	}
	return results
}

func TestPayloads_SingleMarker(t *testing.T) {
	payloads := []string{"admin", "test", "guest"}

	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET /§FUZZ§ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
		Markers: map[string]any{
			"§FUZZ§": payloads,
		},
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var results []fuzzer.FuzzerResult
	go func() {
		defer wg.Done()
		results = collectResults(f)
	}()

	if err := f.Fuzz(); err != nil {
		t.Fatalf("Fuzz() error: %v", err)
	}
	wg.Wait()

	if len(results) != len(payloads) {
		t.Fatalf("expected %d results, got %d", len(payloads), len(results))
	}

	// Results may arrive out of order due to concurrency; check by set membership
	seen := make(map[string]bool)
	for _, r := range results {
		seen[r.Markers["§FUZZ§"]] = true
		expected := fmt.Sprintf("GET /%s HTTP/1.1\r\nHost: example.com\r\n\r\n", r.Markers["§FUZZ§"])
		if r.Request != expected {
			t.Errorf("marker not replaced in request.\nexpected: %q\ngot:      %q", expected, r.Request)
		}
	}
	for _, p := range payloads {
		if !seen[p] {
			t.Errorf("payload %q not found in results", p)
		}
	}
}

func TestPayloads_Progress(t *testing.T) {
	payloads := []string{"a", "b", "c", "d", "e"}

	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET /§P§ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
		Markers: map[string]any{
			"§P§": payloads,
		},
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectResults(f)
	}()

	if err := f.Fuzz(); err != nil {
		t.Fatalf("Fuzz() error: %v", err)
	}
	wg.Wait()

	completed, total := f.GetProgress()
	if total != len(payloads) {
		t.Errorf("expected total=%d, got %d", len(payloads), total)
	}
	if completed != len(payloads) {
		t.Errorf("expected completed=%d, got %d", len(payloads), completed)
	}
}

func TestPayloads_SinglePayload(t *testing.T) {
	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET /§X§ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
		Markers: map[string]any{
			"§X§": []string{"only-one"},
		},
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var results []fuzzer.FuzzerResult
	go func() {
		defer wg.Done()
		results = collectResults(f)
	}()

	if err := f.Fuzz(); err != nil {
		t.Fatalf("Fuzz() error: %v", err)
	}
	wg.Wait()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Markers["§X§"] != "only-one" {
		t.Errorf("expected marker value %q, got %q", "only-one", results[0].Markers["§X§"])
	}
}

func TestPayloads_NoMarkersOrPayloads(t *testing.T) {
	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
	})

	go func() {
		collectResults(f)
	}()

	err := f.Fuzz()
	if err == nil {
		t.Fatal("expected error when no markers provided")
	}
}

func TestPayloads_EmptyPayloadList(t *testing.T) {
	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET /§FUZZ§ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
		Markers: map[string]any{
			"§FUZZ§": []string{},
		},
	})

	go func() {
		collectResults(f)
	}()

	err := f.Fuzz()
	if err == nil {
		t.Fatal("expected error when payload list is empty")
	}
}

func TestPayloads_PayloadWithSpecialChars(t *testing.T) {
	payloads := []string{
		"<script>alert(1)</script>",
		"' OR 1=1 --",
		"../../../etc/passwd",
		"{{template}}",
	}

	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET /§FUZZ§ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
		Markers: map[string]any{
			"§FUZZ§": payloads,
		},
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var results []fuzzer.FuzzerResult
	go func() {
		defer wg.Done()
		results = collectResults(f)
	}()

	if err := f.Fuzz(); err != nil {
		t.Fatalf("Fuzz() error: %v", err)
	}
	wg.Wait()

	if len(results) != len(payloads) {
		t.Fatalf("expected %d results, got %d", len(payloads), len(results))
	}

	seen := make(map[string]bool)
	for _, r := range results {
		seen[r.Markers["§FUZZ§"]] = true
	}
	for _, p := range payloads {
		if !seen[p] {
			t.Errorf("payload %q not found in results", p)
		}
	}
}

func TestMixed_WordlistAndPayloads(t *testing.T) {
	// Create a temporary wordlist file
	wordlistContent := "admin\nroot\nguest\n"
	tmpFile, err := os.CreateTemp("", "wordlist-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(wordlistContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// §USER§ = wordlist file, §PASS§ = inline payloads
	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET /login?user=§USER§&pass=§PASS§ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
		Markers: map[string]any{
			"§USER§": tmpFile.Name(),                            // file path
			"§PASS§": []string{"password1", "123456", "qwerty"}, // inline payloads
		},
		Mode: "pitch_fork",
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var results []fuzzer.FuzzerResult
	go func() {
		defer wg.Done()
		results = collectResults(f)
	}()

	if err := f.Fuzz(); err != nil {
		t.Fatalf("Fuzz() error: %v", err)
	}
	wg.Wait()

	// pitch_fork mode: min(3 users, 3 passwords) = 3 requests
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Results may arrive out of order; verify all expected pairs exist
	expectedPairs := map[string]string{
		"admin": "password1",
		"root":  "123456",
		"guest": "qwerty",
	}

	for _, r := range results {
		user := r.Markers["§USER§"]
		pass := r.Markers["§PASS§"]
		expectedPass, ok := expectedPairs[user]
		if !ok {
			t.Errorf("unexpected user %q in results", user)
			continue
		}
		if pass != expectedPass {
			t.Errorf("user=%q: expected pass=%q, got %q", user, expectedPass, pass)
		}
		delete(expectedPairs, user)
	}
	for user := range expectedPairs {
		t.Errorf("missing result for user %q", user)
	}
}
