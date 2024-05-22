package base_test

import (
	"fmt"
	"testing"

	"github.com/glitchedgitz/grroxy-db/base"
)

var data = map[string]any{
	"id":       123,
	"username": "john",
	"email":    "johndoe@example.com",
	"info": map[string]any{
		"website": "johnisjohn.com",
		"address": "7854 Nalla Supara, Maharastra",
		"nested": map[string]any{
			"one":   "one",
			"two":   "two",
			"three": "three",
		},
	},
	"req": map[string]any{
		"headers": map[string]any{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64)",
		},
	},
}

func TestExtractValueFromMap(t *testing.T) {
	scenarios := []struct {
		key            string
		extractedValue string
		expectedError  bool
	}{
		// Integer
		{`req.headers.User-Agent`, "Mozilla/5.0 (X11; Linux x86_64)", false},
	}

	fmt.Println("    Data    :", data)
	for i, scenario := range scenarios {
		t.Run(fmt.Sprintf("s%d:%s", i, scenario.key), func(t *testing.T) {
			v, err := base.ExtractValueFromMap(&data, scenario.key)

			scenarioInfo := fmt.Sprintf(`
Scenario: ------------------------------------------ %d

Filter  : %s
Result  : %v
Error   : %v

`, i, scenario.key, v, err)

			if scenario.expectedError && err == nil {
				t.Fatalf("Expected error, got nil (%q)\n%v", scenario.key, scenarioInfo)
			}

			if !scenario.expectedError && err != nil {
				t.Fatalf("Did not expect error, got %q (%q).\n%v", err, scenario.key, scenarioInfo)
			}

			if v != scenario.extractedValue {
				t.Fatalf("Expected %v, got %v\n%v", scenario.expectedError, v, scenarioInfo)
			}
		})
	}
}
