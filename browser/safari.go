package browser

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func launchSafari(proxyAddress string, customCertPath string) error {
	log.Println("[launchSafari] Starting Safari launch process")

	// Safari is only available on macOS
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("[launchSafari] Safari is only available on macOS")
	}
	log.Printf("[launchSafari] Running on macOS, proceeding with Safari launch")

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("[launchSafari] failed to get home directory: %v", err)
	}
	log.Printf("[launchSafari] Home directory: %s", homeDir)

	// Create Safari proxy configuration directory
	safariConfigDir := filepath.Join(homeDir, ".proxy-safari")
	log.Printf("[launchSafari] Safari config directory: %s", safariConfigDir)

	if err := os.RemoveAll(safariConfigDir); err != nil {
		log.Printf("[launchSafari] Warning: couldn't clean up old profile: %v", err)
	}
	if err := os.MkdirAll(safariConfigDir, 0755); err != nil {
		return fmt.Errorf("[launchSafari] failed to create Safari config directory: %v", err)
	}
	log.Printf("[launchSafari] Created Safari config directory successfully")

	// Copy CA certificate to config directory for user reference
	certPath := filepath.Join(safariConfigDir, "ca.crt")
	log.Printf("[launchSafari] Copying certificate from %s to %s", customCertPath, certPath)
	if err := copyFile(customCertPath, certPath); err != nil {
		return fmt.Errorf("[launchSafari] failed to copy certificate: %v", err)
	}
	log.Printf("[launchSafari] Certificate copied successfully")

	// Launch Safari
	safariPath := "/Applications/Safari.app/Contents/MacOS/Safari"
	log.Printf("[launchSafari] Using Safari path: %s", safariPath)

	// Verify Safari executable exists
	if _, err := os.Stat(safariPath); err != nil {
		return fmt.Errorf("[launchSafari] Safari executable not found at %s: %v", safariPath, err)
	}
	log.Printf("[launchSafari] Safari executable found and verified")

	// Launch Safari
	log.Printf("[launchSafari] Attempting to launch Safari with about:blank")
	cmd := exec.Command(safariPath, "about:blank")
	log.Printf("[launchSafari] Command: %s", cmd.String())
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[launchSafari] failed to launch Safari: %v", err)
	}

	log.Printf("[launchSafari] Safari process started successfully")
	log.Printf("[launchSafari] IMPORTANT: Safari requires system proxy settings to be configured manually")
	log.Printf("[launchSafari] Please configure your system proxy settings to use %s for HTTP and HTTPS", proxyAddress)
	log.Printf("[launchSafari] The application will not automatically modify your system settings")

	return nil
}
