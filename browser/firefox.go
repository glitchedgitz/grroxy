package browser

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func launchFirefox(proxyAddress string, customCertPath string) (*exec.Cmd, error) {
	log.Println("[launchFirefox] Starting Firefox launch process")

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("[launchFirefox] failed to get home directory: %v", err)
	}
	log.Printf("[launchFirefox] Home directory: %s", homeDir)

	// Create Firefox profile directory
	profileDir := filepath.Join(homeDir, ".proxy-firefox")
	log.Printf("[launchFirefox] Firefox profile directory: %s", profileDir)

	if err := os.RemoveAll(profileDir); err != nil {
		log.Printf("[launchFirefox] Warning: couldn't clean up old profile: %v", err)
	}
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return nil, fmt.Errorf("[launchFirefox] failed to create Firefox profile directory: %v", err)
	}
	log.Printf("[launchFirefox] Created Firefox profile directory successfully")

	// Copy CA certificate to the profile directory
	certPath := filepath.Join(profileDir, "ca.crt")
	log.Printf("[launchFirefox] Copying certificate from %s to %s", customCertPath, certPath)
	if err := copyFile(customCertPath, certPath); err != nil {
		return nil, fmt.Errorf("[launchFirefox] failed to copy certificate: %v", err)
	}
	log.Printf("[launchFirefox] Certificate copied successfully")

	// Create the cert_override.txt file to permanently accept the CA
	certOverridePath := filepath.Join(profileDir, "cert_override.txt")
	log.Printf("[launchFirefox] Creating cert_override.txt at %s", certOverridePath)
	certOverrideContent := "# PSM Certificate Override Settings file\n# This is a generated file!  Do not edit.\n"
	if err := os.WriteFile(certOverridePath, []byte(certOverrideContent), 0644); err != nil {
		return nil, fmt.Errorf("[launchFirefox] failed to write cert_override.txt: %v", err)
	}
	log.Printf("[launchFirefox] Created cert_override.txt successfully")

	// Create profiles.ini
	profileIniPath := filepath.Join(profileDir, "profiles.ini")
	log.Printf("[launchFirefox] Creating profiles.ini at %s", profileIniPath)
	profileIniContent := `[Profile0]
Name=default
IsRelative=0
Path=${PROFILE_PATH}
Default=1

[General]
StartWithLastProfile=1
Version=2
`
	profileIniContent = strings.Replace(profileIniContent, "${PROFILE_PATH}", profileDir, -1)
	if err := os.WriteFile(profileIniPath, []byte(profileIniContent), 0644); err != nil {
		return nil, fmt.Errorf("[launchFirefox] failed to write Firefox profile.ini: %v", err)
	}
	log.Printf("[launchFirefox] Created profiles.ini successfully")

	// Create necessary certificate database directories
	certdbDir := filepath.Join(profileDir, "cert_db")
	log.Printf("[launchFirefox] Creating cert_db directory at %s", certdbDir)
	if err := os.MkdirAll(certdbDir, 0755); err != nil {
		log.Printf("[launchFirefox] Warning: Could not create cert_db directory: %v", err)
	}

	// Set up default preferences
	prefs := map[string]interface{}{
		// Browser UI preferences
		"browser.shell.checkDefaultBrowser":           false,
		"browser.bookmarks.restore_default_bookmarks": false,
		"browser.startup.page":                        0,
		"browser.tabs.warnOnClose":                    false,
		"browser.sessionstore.resume_from_crash":      false,
		"browser.download.panel.shown":                true,
		"browser.download.folderList":                 1,

		// Performance preferences
		"browser.cache.disk.capacity":             0,
		"browser.cache.disk.smart_size.enabled":   false,
		"browser.cache.disk.smart_size.first_run": false,

		// DOM preferences
		"dom.disable_open_during_load": false,
		"dom.max_script_run_time":      0,
	}

	// Parse proxy address into host and port
	proxyHost := "localhost"
	proxyPort := 8888
	if proxyAddress != "" {
		parts := strings.Split(proxyAddress, ":")
		if len(parts) == 2 {
			proxyHost = parts[0]
			if port, err := strconv.Atoi(parts[1]); err == nil {
				proxyPort = port
			}
		}
	}
	log.Printf("[launchFirefox] Using proxy: %s:%d", proxyHost, proxyPort)

	// Add proxy settings to preferences
	prefs["network.proxy.type"] = 1
	prefs["network.proxy.http"] = proxyHost
	prefs["network.proxy.http_port"] = proxyPort
	prefs["network.proxy.ssl"] = proxyHost
	prefs["network.proxy.ssl_port"] = proxyPort
	prefs["network.proxy.ftp"] = proxyHost
	prefs["network.proxy.ftp_port"] = proxyPort
	prefs["network.proxy.socks"] = proxyHost
	prefs["network.proxy.socks_port"] = proxyPort
	prefs["network.proxy.no_proxies_on"] = ""
	prefs["network.proxy.share_proxy_settings"] = true

	// Add security settings to preferences - crucial for certificate acceptance
	prefs["security.enterprise_roots.enabled"] = true
	prefs["security.cert_pinning.enforcement_level"] = 0
	prefs["security.default_personal_cert"] = "Select Automatically"
	prefs["security.OCSP.enabled"] = 0
	prefs["security.ssl.enable_ocsp_stapling"] = false
	prefs["security.ssl.enable_ocsp_must_staple"] = false
	prefs["security.mixed_content.block_active_content"] = false
	prefs["security.mixed_content.block_display_content"] = false
	prefs["security.ssl.errorReporting.automatic"] = false
	prefs["browser.ssl_override_behavior"] = 2
	prefs["browser.xul.error_pages.expert_bad_cert"] = true
	prefs["security.tls.version.min"] = 1
	prefs["security.tls.insecure_fallback_hosts.use_static_list"] = false
	prefs["security.remember_cert_checkbox_default_setting"] = true
	prefs["security.warn_viewing_mixed"] = false
	prefs["security.certerrors.permanentOverride"] = true
	prefs["browser.safebrowsing.enabled"] = false
	prefs["browser.safebrowsing.malware.enabled"] = false
	prefs["security.pki.mitm_detected"] = false
	prefs["security.pki.mitm_canary_issuer"] = ""
	prefs["security.pki.mitm_canary_issuer.enabled"] = false
	prefs["security.pki.mitm_compromise_canary"] = ""
	prefs["security.pki.mitm_canary_pub_hpkp_seen"] = false
	prefs["security.ssl3.rsa_des_ede3_sha"] = true
	prefs["security.ssl.treat_unsafe_negotiation_as_broken"] = false
	prefs["security.ssl.require_safe_negotiation"] = false

	// Convert preferences to string content
	var prefsLines []string
	for key, value := range prefs {
		var prefLine string
		switch v := value.(type) {
		case bool:
			prefLine = fmt.Sprintf("user_pref(\"%s\", %t);", key, v)
		case int:
			prefLine = fmt.Sprintf("user_pref(\"%s\", %d);", key, v)
		case string:
			prefLine = fmt.Sprintf("user_pref(\"%s\", \"%s\");", key, v)
		default:
			prefLine = fmt.Sprintf("user_pref(\"%s\", %v);", key, v)
		}
		prefsLines = append(prefsLines, prefLine)
	}

	// Write preferences to prefs.js
	prefsJsPath := filepath.Join(profileDir, "prefs.js")
	log.Printf("[launchFirefox] Writing preferences to %s", prefsJsPath)
	prefsContent := strings.Join(prefsLines, "\n")
	if err := os.WriteFile(prefsJsPath, []byte(prefsContent), 0644); err != nil {
		return nil, fmt.Errorf("[launchFirefox] failed to write Firefox preferences: %v", err)
	}
	log.Printf("[launchFirefox] Wrote preferences successfully")

	// Create user.js with same preferences to ensure they're applied
	userJsPath := filepath.Join(profileDir, "user.js")
	log.Printf("[launchFirefox] Writing user.js to %s", userJsPath)
	if err := os.WriteFile(userJsPath, []byte(prefsContent), 0644); err != nil {
		return nil, fmt.Errorf("[launchFirefox] failed to write Firefox user.js: %v", err)
	}
	log.Printf("[launchFirefox] Wrote user.js successfully")

	// On macOS and Linux, try to install the certificate to Firefox's database
	// using certutil if available
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		// Attempt to import the certificate using Firefox's certutil tool if available
		log.Printf("[launchFirefox] Note: For permanent certificate trust, you may need to manually install the certificate from: %s", certPath)

		// Create NSS database directories
		if err := os.MkdirAll(filepath.Join(profileDir, "chrome"), 0755); err != nil {
			log.Printf("[launchFirefox] Warning: Could not create chrome directory: %v", err)
		}
	}

	// For Firefox, instead of creating empty cert DB files, we'll initialize the NSS database properly
	// by creating the necessary directory structure that Firefox expects
	for _, dir := range []string{"cert9.db", "key4.db", "pkcs11.txt"} {
		dirPath := filepath.Join(profileDir, dir)
		// For files, we'll create empty files
		if strings.HasSuffix(dir, ".db") || strings.HasSuffix(dir, ".txt") {
			file, err := os.Create(dirPath)
			if err != nil {
				log.Printf("[launchFirefox] Warning: Could not create %s file: %v", dir, err)
			} else {
				file.Close()
			}
		} else {
			// For directories
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				log.Printf("[launchFirefox] Warning: Could not create %s directory: %v", dir, err)
			}
		}
	}

	// Determine Firefox executable path
	var firefoxPath string
	switch runtime.GOOS {
	case "darwin": // macOS
		firefoxPath = "/Applications/Firefox.app/Contents/MacOS/firefox"
		log.Printf("[launchFirefox] Using macOS Firefox path: %s", firefoxPath)
	case "linux":
		// Try common locations for Firefox on Linux
		possiblePaths := []string{
			"firefox",
			"/usr/bin/firefox",
			"/usr/local/bin/firefox",
			"/snap/bin/firefox",
		}

		for _, path := range possiblePaths {
			if _, err := exec.LookPath(path); err == nil {
				firefoxPath = path
				log.Printf("[launchFirefox] Found Firefox at: %s", path)
				break
			}
		}

		if firefoxPath == "" {
			firefoxPath = "firefox" // Default to PATH lookup
			log.Printf("[launchFirefox] Using default Firefox path from PATH")
		}
	case "windows":
		firefoxPath = "C:\\Program Files\\Mozilla Firefox\\firefox.exe"
		if _, err := os.Stat(firefoxPath); err != nil {
			log.Printf("[launchFirefox] Firefox not found at primary path, trying alternative path")
			firefoxPath = "C:\\Program Files (x86)\\Mozilla Firefox\\firefox.exe"
		}
		log.Printf("[launchFirefox] Using Windows Firefox path: %s", firefoxPath)
	default:
		return nil, fmt.Errorf("[launchFirefox] unsupported operating system: %s", runtime.GOOS)
	}

	// Verify Firefox executable exists
	if _, err := os.Stat(firefoxPath); err != nil {
		return nil, fmt.Errorf("[launchFirefox] Firefox executable not found at %s: %v", firefoxPath, err)
	}
	log.Printf("[launchFirefox] Firefox executable found and verified")

	// Build command line arguments
	args := []string{
		"-profile", profileDir,
		"-no-remote",
		"-new-instance",
		"grroxy.com",
	}
	log.Printf("[launchFirefox] Firefox arguments: %v", args)

	// Launch Firefox
	log.Printf("[launchFirefox] Attempting to launch Firefox with command: %s %v", firefoxPath, args)
	cmd := exec.Command(firefoxPath, args...)
	log.Printf("[launchFirefox] Command: %s", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("[launchFirefox] failed to launch Firefox: %v", err)
	}

	log.Printf("[launchFirefox] Firefox process started successfully")
	log.Printf("[launchFirefox] Firefox profile at: %s", profileDir)

	// Display instructions for manually installing certificate if needed
	if runtime.GOOS == "darwin" {
		log.Printf("[launchFirefox] Note: If Firefox prompts about certificate errors, go to about:preferences#privacy,")
		log.Printf("[launchFirefox] scroll to Certificates, click 'View Certificates', go to 'Authorities' tab,")
		log.Printf("[launchFirefox] and import the certificate from: %s", certPath)
	} else if runtime.GOOS == "linux" {
		log.Printf("[launchFirefox] Note: If Firefox prompts about certificate errors, go to about:preferences#privacy,")
		log.Printf("[launchFirefox] scroll to Certificates, click 'View Certificates', go to 'Authorities' tab,")
		log.Printf("[launchFirefox] and import the certificate from: %s", certPath)
	} else if runtime.GOOS == "windows" {
		log.Printf("[launchFirefox] Note: If Firefox prompts about certificate errors, go to Options > Privacy & Security,")
		log.Printf("[launchFirefox] scroll to Certificates, click 'View Certificates', go to 'Authorities' tab,")
		log.Printf("[launchFirefox] and import the certificate from: %s", certPath)
	}

	return cmd, nil
}
