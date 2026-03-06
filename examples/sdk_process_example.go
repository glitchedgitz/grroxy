package main

import (
	"fmt"
	"log"
	"time"

	"github.com/glitchedgitz/grroxy/internal/sdk"
)

// Example: How to use the SDK to manage processes from external tools like grroxy-tool

func main() {
	// Initialize the SDK client
	client := sdk.NewClient(
		"http://localhost:8090",
		sdk.WithAdminEmailPassword("admin@example.com", "password"),
		// sdk.WithDebug(), // Uncomment for debug mode
	)

	// Example 1: Create a fuzzer process
	fuzzerExample(client)

	// Example 2: Create a custom tool process
	customToolExample(client)
}

func fuzzerExample(client *sdk.Client) {
	fmt.Println("=== Fuzzer Process Example ===")

	// Create a new fuzzer process
	processID, err := client.CreateProcess(sdk.CreateProcessRequest{
		Name:        "Fuzzer - example.com",
		Description: "Fuzzing example.com with wordlist",
		Type:        "fuzzer",
		State:       "running",
		Data: map[string]any{
			"host":     "example.com",
			"wordlist": "/path/to/wordlist.txt",
			"markers":  map[string]string{"§FUZZ§": "/path/to/wordlist.txt"},
		},
		Input: &sdk.ProcessInput{
			Completed: 0,
			Total:     1000,
			Progress:  0,
			Message:   "Starting fuzzer...",
			Error:     "",
		},
	})

	if err != nil {
		log.Fatalf("Failed to create process: %v", err)
	}

	fmt.Printf("Created process with ID: %s\n", processID)

	// Simulate fuzzing progress
	for i := 0; i <= 1000; i += 100 {
		time.Sleep(1 * time.Second)

		err = client.UpdateProcess(processID, sdk.ProgressUpdate{
			Completed: i,
			Total:     1000,
			Message:   fmt.Sprintf("Processing request %d/1000", i),
			State:     "running",
		})

		if err != nil {
			log.Printf("Failed to update progress: %v", err)
		}
	}

	// Complete the process
	err = client.CompleteProcess(processID, "Fuzzing completed successfully")
	if err != nil {
		log.Printf("Failed to complete process: %v", err)
	}

	fmt.Println("Fuzzer process completed!")
}

func customToolExample(client *sdk.Client) {
	fmt.Println("\n=== Custom Tool Process Example ===")

	// Create a custom tool process
	processID, err := client.CreateProcess(sdk.CreateProcessRequest{
		Name:        "Custom Scanner",
		Description: "Scanning target with custom tool",
		Type:        "scanner",
		Data: map[string]any{
			"target": "https://example.com",
			"depth":  3,
		},
	})

	if err != nil {
		log.Fatalf("Failed to create process: %v", err)
	}

	fmt.Printf("Created process with ID: %s\n", processID)

	// Simulate scanning
	totalPages := 50
	for i := 0; i <= totalPages; i += 5 {
		time.Sleep(500 * time.Millisecond)

		err = client.UpdateProcess(processID, sdk.ProgressUpdate{
			Completed: i,
			Total:     totalPages,
			Message:   fmt.Sprintf("Scanned %d/%d pages", i, totalPages),
		})

		if err != nil {
			log.Printf("Failed to update progress: %v", err)
		}
	}

	// Complete the process
	err = client.CompleteProcess(processID, fmt.Sprintf("Scanned %d pages successfully", totalPages))
	if err != nil {
		log.Printf("Failed to complete process: %v", err)
	}

	fmt.Println("Scanner process completed!")
}

// Example: Handling errors
func errorHandlingExample(client *sdk.Client) {
	fmt.Println("\n=== Error Handling Example ===")

	processID, err := client.CreateProcess(sdk.CreateProcessRequest{
		Name: "Failing Process",
		Type: "test",
	})

	if err != nil {
		log.Fatalf("Failed to create process: %v", err)
	}

	// Simulate some work
	time.Sleep(1 * time.Second)

	// Something went wrong, mark as failed
	err = client.FailProcess(processID, "Connection timeout after 30 seconds")
	if err != nil {
		log.Printf("Failed to mark process as failed: %v", err)
	}

	fmt.Println("Process marked as failed")
}

// Example: Pausing and killing processes
func pauseKillExample(client *sdk.Client) {
	fmt.Println("\n=== Pause/Kill Example ===")

	processID, err := client.CreateProcess(sdk.CreateProcessRequest{
		Name: "Long Running Process",
		Type: "background",
	})

	if err != nil {
		log.Fatalf("Failed to create process: %v", err)
	}

	// Pause the process
	err = client.PauseProcess(processID, "User requested pause")
	if err != nil {
		log.Printf("Failed to pause process: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Kill the process
	err = client.KillProcess(processID, "User cancelled operation")
	if err != nil {
		log.Printf("Failed to kill process: %v", err)
	}

	fmt.Println("Process killed")
}
