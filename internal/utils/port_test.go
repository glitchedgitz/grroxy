package utils

import (
	"fmt"
	"testing"
)

// Create test for FindAvailablePort
func TestCheckAndFindAvailablePort(t *testing.T) {
	// Create test cases
	tests := []struct {
		host string
	}{
		{"127.0.0.1:8080"},
		{"127.0.0.1:15292"},
	}

	// Loop through test cases
	for _, test := range tests {
		// Call FindAvailablePort
		result, err := CheckAndFindAvailablePort(test.host)

		fmt.Println("Port found: ", result)

		// Check for error
		if err != nil {
			t.Errorf("Error finding available port: %v", err)
		}
	}

}
