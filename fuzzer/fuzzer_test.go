package fuzzer_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/glitchedgitz/grroxy-db/fuzzer"
)

func TestFuzzer(t *testing.T) {
	t.Log("[fuzzer] starting test")

	var wg sync.WaitGroup

	f := fuzzer.NewFuzzer(fuzzer.FuzzerConfig{
		Request: "GET /FUZZ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		Host:    "example.com",
		Markers: map[string]string{
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
