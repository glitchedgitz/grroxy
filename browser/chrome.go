package browser

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func launchChrome(proxyAddress string, customCertPath string) error {
	log.Println("[launchChrome] Starting Chrome launch process")

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("[launchChrome] failed to get home directory: %v", err)
	}
	log.Printf("[launchChrome] Home directory: %s", homeDir)

	// Create Chrome user data directory
	chromeDataDir := filepath.Join(homeDir, ".proxy-chrome")
	log.Printf("[launchChrome] Chrome data directory: %s", chromeDataDir)

	if err := os.RemoveAll(chromeDataDir); err != nil {
		log.Printf("[launchChrome] Warning: couldn't clean up old profile: %v", err)
	}
	if err := os.MkdirAll(chromeDataDir, 0755); err != nil {
		return fmt.Errorf("[launchChrome] failed to create Chrome data directory: %v", err)
	}
	log.Printf("[launchChrome] Created Chrome data directory successfully")

	// Copy CA certificate to Chrome's certificate store
	certPath := filepath.Join(chromeDataDir, "ca.crt")
	log.Printf("[launchChrome] Copying certificate from %s to %s", customCertPath, certPath)
	if err := copyFile(customCertPath, certPath); err != nil {
		return fmt.Errorf("[launchChrome] failed to copy certificate: %v", err)
	}
	log.Printf("[launchChrome] Certificate copied successfully")

	// Calculate the SHA-256 SPKI fingerprint of our root CA
	log.Printf("[launchChrome] Calculating certificate fingerprint")
	fingerprint, err := GetSPKIFingerprint(certPath)
	if err != nil {
		log.Printf("[launchChrome] Warning: couldn't calculate certificate fingerprint: %v", err)
		log.Printf("[launchChrome] Certificate trust may not work correctly")
	} else {
		log.Printf("[launchChrome] Certificate fingerprint calculated successfully")
	}

	// Determine Chrome executable path
	var chromePath string
	switch runtime.GOOS {
	case "darwin": // macOS
		chromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(chromePath); err != nil {
			log.Printf("[launchChrome] Chrome not found at primary path, trying Chromium")
			chromePath = "/Applications/Chromium.app/Contents/MacOS/Chromium"
		}
	case "linux":
		chromePath = "google-chrome"
	case "windows":
		chromePath = "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"
		if _, err := os.Stat(chromePath); err != nil {
			log.Printf("[launchChrome] Chrome not found at primary path, trying alternative path")
			chromePath = "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe"
		}
	default:
		return fmt.Errorf("[launchChrome] unsupported operating system: %s", runtime.GOOS)
	}
	log.Printf("[launchChrome] Using Chrome path: %s", chromePath)

	// Verify Chrome executable exists
	if _, err := os.Stat(chromePath); err != nil {
		return fmt.Errorf("[launchChrome] Chrome executable not found at %s: %v", chromePath, err)
	}
	log.Printf("[launchChrome] Chrome executable found and verified")

	// Construct Chrome command line arguments
	args := []string{
		"--user-data-dir=" + chromeDataDir,
		"--proxy-server=" + proxyAddress,
	}

	// Add certificate fingerprint to the ignore list if we were able to calculate it
	if fingerprint != "" {
		args = append(args, "--ignore-certificate-errors-spki-list="+fingerprint)
	} else {
		// Fallback to the older, less secure method
		args = append(args, "--ignore-certificate-errors")
	}

	// Add other standard arguments
	args = append(args,
		"--allow-insecure-localhost",
		"--unsafely-treat-insecure-origin-as-secure",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-restore-session-state",
		"--disable-popup-blocking",
		"--disable-translate",
		"--disable-infobars",
		"--enable-features=SuppressDifferentOriginSubframeDialogs",
		"--disable-extensions-except=",
		"--disable-component-extensions-with-background-pages",
		"--start-maximized",
		"--disable-default-apps",
		"--disable-sync",
		"--enable-fixed-layout",
		"--noerrdialogs",
		"--test-type",
		"grroxy.com",
	)

	log.Printf("[launchChrome] Chrome arguments: %v", args)

	// Launch Chrome
	log.Printf("[launchChrome] Attempting to launch Chrome with command: %s %v", chromePath, args)
	cmd := exec.Command(chromePath, args...)
	log.Println("[launchChrome] " + cmd.String())
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[launchChrome] failed to launch Chrome: %v", err)
	}

	log.Printf("[launchChrome] Chrome process started successfully")
	log.Printf("[launchChrome] Chrome profile at: %s", chromeDataDir)
	return nil
}
