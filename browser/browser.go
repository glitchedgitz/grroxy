package browser

import (
	"fmt"
	"os"
	"strings"
)

func LaunchBrowser(browserType string, proxyAddress string, customCertPath string) error {
	if browserType == "" {
		browserType = "chrome" // Default to Chrome
	}

	browserType = strings.ToLower(browserType)

	switch browserType {
	case "chrome":
		return launchChrome(proxyAddress, customCertPath)
	case "firefox":
		return launchFirefox(proxyAddress, customCertPath)
	case "safari":
		return launchSafari(proxyAddress, customCertPath)
	default:
		return fmt.Errorf("unsupported browser: %s", browserType)
	}
}

// copyFile is kept in this file as it's used by all browser implementations
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, input, 0644)
}
